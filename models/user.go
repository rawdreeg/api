package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/olivere/elastic/v7"
	"golang.org/x/crypto/bcrypt"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/queue"
	og "github.com/hiconvo/api/utils/opengraph"
	"github.com/hiconvo/api/utils/random"
)

type User struct {
	client *Client `datastore:"-"`

	Key              *datastore.Key   `json:"-"        datastore:"__key__"`
	ID               string           `json:"id"       datastore:"-"`
	Email            string           `json:"email"`
	Emails           []string         `json:"emails"`
	FirstName        string           `json:"firstName"`
	LastName         string           `json:"lastName"`
	FullName         string           `json:"fullName" datastore:"-"`
	Token            string           `json:"token"`
	RealtimeToken    string           `json:"realtimeToken"`
	PasswordDigest   string           `json:"-"        datastore:",noindex"`
	OAuthGoogleID    string           `json:"-"`
	OAuthFacebookID  string           `json:"-"`
	IsPasswordSet    bool             `json:"isPasswordSet"    datastore:"-"`
	IsGoogleLinked   bool             `json:"isGoogleLinked"   datastore:"-"`
	IsFacebookLinked bool             `json:"isFacebookLinked" datastore:"-"`
	IsLocked         bool             `json:"-"`
	Verified         bool             `json:"verified"`
	Avatar           string           `json:"avatar"`
	ContactKeys      []*datastore.Key `json:"-"`
	Contacts         []*UserPartial   `json:"-"        datastore:"-"`
	CreatedAt        time.Time        `json:"-"`
}

func (c *Client) NewIncompleteUser(email string) (User, error) {
	femail := strings.ToLower(email)

	user := User{
		client: c,

		Key:       datastore.IncompleteKey("User", nil),
		Email:     femail,
		FirstName: strings.Split(femail, "@")[0],
		Token:     random.Token(),
		Verified:  false,
		CreatedAt: time.Now(),
	}

	return user, nil
}

func (c *Client) NewUserWithPassword(email, firstname, lastname, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return User{}, err
	}

	femail := strings.ToLower(email)

	user := User{
		client: c,

		Key:             datastore.IncompleteKey("User", nil),
		Email:           femail,
		FirstName:       firstname,
		LastName:        lastname,
		FullName:        "",
		PasswordDigest:  string(hash),
		Token:           random.Token(),
		OAuthGoogleID:   "",
		OAuthFacebookID: "",
		Verified:        false,
		CreatedAt:       time.Now(),
	}

	return user, nil
}

func (c *Client) NewUserWithOAuth(email, firstname, lastname, avatar, oauthprovider, oauthtoken string) (User, error) {
	var googleID string
	var facebookID string
	if oauthprovider == "google" {
		googleID = oauthtoken
	} else {
		facebookID = oauthtoken
	}

	femail := strings.ToLower(email)

	user := User{
		client: c,

		Key:             datastore.IncompleteKey("User", nil),
		Email:           femail,
		Emails:          []string{femail},
		FirstName:       firstname,
		LastName:        lastname,
		FullName:        "",
		Avatar:          avatar,
		PasswordDigest:  "",
		Token:           random.Token(),
		OAuthGoogleID:   googleID,
		OAuthFacebookID: facebookID,
		Verified:        true,
		CreatedAt:       time.Now(),
	}

	return user, nil
}

func (u *User) LoadKey(k *datastore.Key) error {
	u.Key = k

	// Add URL safe key
	u.ID = k.Encode()

	return nil
}

func (u *User) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(u)
}

func (u *User) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(u, ps); err != nil {
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if !(mismatch.FieldName == "Threads" || mismatch.FieldName == "Events") {
				return err
			}
		} else {
			return err
		}
	}

	u.DeriveProperties()

	return nil
}

func (u *User) Commit(ctx context.Context) error {
	// Add CreatedAt date
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	// If the user has both first and last names, capitalize them
	// properly
	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(u.FirstName)
		u.LastName = strings.Title(u.LastName)
	}

	// Trim whitespace around the user's names
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)

	key, err := u.client.db.Put(ctx, u.Key, u)
	if err != nil {
		return err
	}

	u.ID = key.Encode()
	u.Key = key

	// We have to do this after the user has been saved because we need the
	// ID, which isn't available until the user is in the database
	u.RealtimeToken = u.client.ntf.GenerateToken(u.ID)

	u.DeriveProperties()
	u.CreateOrUpdateSearchIndex(ctx)

	return nil
}

func (u *User) CommitWithTransaction(tx db.Transaction) (*datastore.PendingKey, error) {
	return tx.Put(u.Key, u)
}

func (u *User) CreateOrUpdateSearchIndex(ctx context.Context) {
	if u.IsRegistered() {
		_, upsertErr := u.client.search.Update().
			Index("users").
			Id(u.ID).
			DocAsUpsert(true).
			Doc(MapUserToUserPartial(u)).
			Do(ctx)
		if upsertErr != nil {
			log.Printf("Failed to index user in elasticsearch: %v", upsertErr)
		}
	}
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordDigest), []byte(password))
	return err == nil
}

func (u *User) ChangePassword(password string) bool {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return false
	}

	u.PasswordDigest = string(hash)

	return true
}

func (u *User) DeriveProperties() {
	// This is repeated from Commit above and handles getting users whose
	// names are improperly formatted. Eventaually this should be removed.
	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(u.FirstName)
		u.LastName = strings.Title(u.LastName)
	}
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)

	// Derive the full name
	u.FullName = strings.TrimSpace(fmt.Sprintf("%s %s", u.FirstName, u.LastName))

	// Derive useful bools
	u.IsPasswordSet = u.PasswordDigest != ""
	u.IsGoogleLinked = u.OAuthGoogleID != ""
	u.IsFacebookLinked = u.OAuthFacebookID != ""

	// For handling transition from single to multi-email model. If the single email was
	// verified, add it to the users Emails list.
	if u.Verified && !u.HasEmail(u.Email) {
		u.AddEmail(u.Email)
	}

	// Make the user's primary email one that is verified if possible.
	if !u.Verified && !u.HasEmail(u.Email) && len(u.Emails) > 0 {
		u.Email = u.Emails[0]
	}

	u.Verified = u.HasEmail(u.Email)
}

func (u *User) IsRegistered() bool {
	return (u.IsGoogleLinked || u.IsFacebookLinked || u.IsPasswordSet) && u.Verified
}

func (u *User) SendPasswordResetEmail() error {
	magicLink := u.client.magic.NewLink(u.Key, u.PasswordDigest, "reset")
	return u.client.mail.SendPasswordResetEmail(u, magicLink)
}

func (u *User) SendVerifyEmail(email string) error {
	femail := strings.ToLower(email)
	salt := femail + strconv.FormatBool(u.HasEmail(femail))
	magicLink := u.client.magic.NewLink(u.Key, salt, "verify/"+femail)
	return u.client.mail.SendVerifyEmail(u, email, magicLink)
}

func (u *User) SendMergeAccountsEmail(emailToMerge string) error {
	femail := strings.ToLower(emailToMerge)
	salt := femail + strconv.FormatBool(u.HasEmail(femail))
	magicLink := u.client.magic.NewLink(u.Key, salt, "verify/"+femail)
	return u.client.mail.SendMergeAccountsEmail(u, femail, magicLink)
}

func (u *User) AddContact(c *User) error {
	var op errors.Op = "models.AddContact"

	if u.HasContact(c) {
		return errors.E(op, http.StatusBadRequest, map[string]string{
			"message": "You already have this contact"})
	}

	if u.Key.Equal(c.Key) {
		return errors.E(op, http.StatusBadRequest, map[string]string{
			"message": "You cannot add yourself as a contact"})
	}

	if len(u.ContactKeys) >= 50 {
		return errors.E(op, http.StatusBadRequest, map[string]string{
			"message": "You can have a maximum of 50 contacts"})
	}

	u.ContactKeys = append(u.ContactKeys, c.Key)

	return nil
}

func (u *User) RemoveContact(c *User) error {
	if !u.HasContact(c) {
		return errors.E(
			errors.Op("user.RemoveContact"),
			http.StatusBadRequest,
			map[string]string{"message": "You don't have this contact"})
	}

	for i, k := range u.ContactKeys {
		if k.Equal(c.Key) {
			u.ContactKeys[i] = u.ContactKeys[len(u.ContactKeys)-1]
			u.ContactKeys = u.ContactKeys[:len(u.ContactKeys)-1]
			return nil
		}
	}

	return nil
}

func (u *User) HasContact(c *User) bool {
	for _, k := range u.ContactKeys {
		if k.Equal(c.Key) {
			return true
		}
	}

	return false
}

// HasEmail returns true when the user has the given email and it it verified.
func (u *User) HasEmail(email string) bool {
	femail := strings.ToLower(email)

	for i := range u.Emails {
		if u.Emails[i] == femail {
			return true
		}
	}

	return false
}

// AddEmail adds a verified email to the user's Emails field. Only verified emails
// should be added.
func (u *User) AddEmail(email string) {
	femail := strings.ToLower(email)

	if u.HasEmail(femail) {
		return
	}

	u.Emails = append(u.Emails, femail)
}

// RemoveEmail removes the given email from the user's Emails field, if the email is present.
func (u *User) RemoveEmail(email string) error {
	femail := strings.ToLower(email)

	if !u.HasEmail(email) {
		return nil
	}

	if u.Email == femail {
		return errors.E(errors.Op("models.RemoveEmail"), http.StatusBadRequest, map[string]string{
			"message": "You cannot remove your primary email"})
	}

	for i := range u.Emails {
		if u.Emails[i] == femail {
			u.Emails[i] = u.Emails[len(u.Emails)-1]
			u.Emails = u.Emails[:len(u.Emails)-1]
			break
		}
	}

	return nil
}

func (u *User) MakeEmailPrimary(email string) error {
	if !u.HasEmail(email) {
		return errors.E(errors.Op("models.MakeEmailPrimary"), http.StatusBadRequest, map[string]string{
			"message": "You cannot make an unverified email primary"})
	}

	u.Email = strings.ToLower(email)
	u.Verified = true

	return nil
}

func (u *User) Welcome(ctx context.Context) {
	var op errors.Op = "user.Welcome"

	thread, err := u.client.NewThread("Welcome", u.client.supportUser, []*User{u})
	if err != nil {
		log.Alarm(errors.E(op, err))
		return
	}

	if err := thread.Commit(ctx); err != nil {
		log.Alarm(errors.E(op, err))
		return
	}

	if err := u.Commit(ctx); err != nil {
		log.Alarm(errors.E(op, err))
		return
	}

	message, err := u.client.NewThreadMessage(u.client.supportUser, &thread, u.client.welcomeMessage, "", og.LinkData{})
	if err != nil {
		log.Alarm(errors.E(op, err))
		return
	}

	if err := message.Commit(ctx); err != nil {
		log.Alarm(errors.E(op, err))
		return
	}

	// We have to save the thread again, which is annoying
	if err := thread.Commit(ctx); err != nil {
		log.Alarm(errors.E(op, err))
		return
	}
}

func (u *User) SendDigest(ctx context.Context) error {
	events, err := u.client.GetEventsByUser(ctx, u, &Pagination{})
	if err != nil {
		return err
	}

	threads, err := u.client.GetThreadsByUser(ctx, u, &Pagination{})
	if err != nil {
		return err
	}

	// Convert the events into Digestables and filter out read items
	var digestables []Digestable
	// Save the upcoming events to a slice at the same time
	var upcoming []*Event
	for i := range events {
		if !IsRead(events[i], u.Key) {
			digestables = append(digestables, events[i])
		}

		if events[i].IsUpcoming() {
			upcoming = append(upcoming, events[i])
		}
	}

	for i := range threads {
		if !IsRead(threads[i], u.Key) {
			digestables = append(digestables, threads[i])
		}
	}

	digestList, err := u.client.GenerateDigestList(ctx, digestables, u)
	if err != nil {
		return err
	}

	if len(digestList) > 0 || len(upcoming) > 0 {
		if err := u.client.mail.SendDigest(digestList, upcoming, u); err != nil {
			return err
		}

		if err := u.client.MarkDigestedMessagesAsRead(ctx, digestList, u); err != nil {
			return err
		}
	}

	return nil
}

func (u *User) MergeWith(ctx context.Context, oldUser *User) error {
	if u.Key.Incomplete() {
		return errors.Str("models.MergeWith: user's key is incomplete")
	}

	if oldUser.Key.Incomplete() {
		return errors.Str("models.MergeWith: oldUser's key is incomplete")
	}

	_, err := u.client.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		// Contacts
		err := u.client.reassignContacts(ctx, tx, oldUser, u)
		if err != nil {
			return err
		}

		// Messages
		err = u.client.reassignMessageUsers(ctx, tx, oldUser, u)
		if err != nil {
			return err
		}

		// Threads
		err = u.client.reassignThreadUsers(ctx, tx, oldUser, u)
		if err != nil {
			return err
		}

		// Events
		err = u.client.reassignEventUsers(ctx, tx, oldUser, u)
		if err != nil {
			return err
		}

		// User details
		if oldUser.Avatar != "" && u.Avatar == "" {
			u.Avatar = oldUser.Avatar
		}
		if oldUser.FirstName != "" && u.FirstName == "" {
			u.FirstName = oldUser.FirstName
		}
		if oldUser.LastName != "" && u.LastName == "" {
			u.LastName = oldUser.LastName
		}
		for _, email := range oldUser.Emails {
			u.AddEmail(email)
		}
		u.ContactKeys = u.client.mergeContacts(u.ContactKeys, oldUser.ContactKeys)
		u.RemoveContact(oldUser)

		// Save user
		_, err = tx.Put(u.Key, u)
		if err != nil {
			return err
		}

		// Remove the old user from search
		if oldUser.IsRegistered() {
			_, err = u.client.search.Delete().
				Index("users").
				Id(oldUser.ID).
				Do(ctx)
			if err != nil {
				log.Alarm(errors.Errorf("Failed to remove user from elasticsearch: %v", err))
			}
		}

		// Delete the old user
		err = tx.Delete(oldUser.Key)
		if err != nil {
			tx.Rollback()
			return err
		}

		return nil
	})

	if err != nil {
		u.CreateOrUpdateSearchIndex(ctx)
	}

	return err
}

func (c *Client) UserSearch(ctx context.Context, query string) ([]UserPartial, error) {
	skip := 0
	take := 10

	contacts := make([]UserPartial, 0)

	esQuery := elastic.NewMultiMatchQuery(query, "fullName", "firstName", "lastName").
		Fuzziness("3").
		MinimumShouldMatch("0")
	result, err := c.search.Search().
		Index("users").
		Query(esQuery).
		From(skip).Size(take).
		Do(ctx)
	if err != nil {
		return contacts, err
	}

	for _, hit := range result.Hits.Hits {
		var contact UserPartial
		jsonErr := json.Unmarshal(hit.Source, &contact)
		if jsonErr != nil {
			return contacts, jsonErr
		}

		contacts = append(contacts, contact)
	}

	return contacts, nil
}

func (c *Client) UserWelcomeMulti(ctx context.Context, users []User) {
	ids := make([]string, len(users))
	for i := range users {
		ids[i] = users[i].ID
	}

	c.queue.PutEmail(ctx, queue.EmailPayload{
		Type:   queue.User,
		Action: queue.SendWelcome,
		IDs:    ids,
	})
}

func (c *Client) GetUserByID(ctx context.Context, id string) (User, error) {
	u := User{}

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return u, err
	}

	if err := c.db.Get(ctx, key, &u); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return u, errors.E(errors.Op("models.GetUserByID"), http.StatusNotFound, err)
		}

		return u, err
	}

	u.client = c

	return u, nil
}

func (c *Client) GetUserByEmail(ctx context.Context, email string) (User, bool, error) {
	femail := strings.ToLower(email)

	u, found, err := c.getUserByField(ctx, "Email", femail)
	if !found && err == nil {
		return c.getUserByField(ctx, "Emails", femail)
	}

	return u, found, err
}

func (c *Client) GetUserByToken(ctx context.Context, token string) (User, bool, error) {
	return c.getUserByField(ctx, "Token", token)
}

func (c *Client) GetUserByOAuthID(ctx context.Context, oauthtoken, provider string) (User, bool, error) {
	if provider == "google" {
		return c.getUserByField(ctx, "OAuthGoogleID", oauthtoken)
	}

	return c.getUserByField(ctx, "OAuthFacebookID", oauthtoken)
}

func (c *Client) GetUsersByThread(ctx context.Context, t *Thread) ([]*User, error) {
	var userKeys []*datastore.Key
	copy(userKeys, t.UserKeys)
	userKeys = append(userKeys, t.OwnerKey)

	users := make([]*User, len(userKeys))
	if err := c.db.GetMulti(ctx, userKeys, users); err != nil {
		return users, err
	}

	for i := range users {
		users[i].client = c
	}

	return users, nil
}

func (c *Client) GetOrCreateUserByEmail(ctx context.Context, email string) (User, bool, error) {
	u, found, err := c.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, false, err
	} else if found {
		return u, false, nil
	}

	u, err = c.NewIncompleteUser(email)
	if err != nil {
		return User{}, false, err
	}

	return u, true, nil
}

func (c *Client) getUserByField(ctx context.Context, field, value string) (User, bool, error) {
	var users []User

	q := datastore.NewQuery("User").Filter(fmt.Sprintf("%s =", field), value)

	keys, getErr := c.db.GetAll(ctx, q, &users)

	if getErr != nil {
		return User{}, false, getErr
	}

	if len(keys) == 1 {
		user := users[0]
		user.client = c

		// Generate the streamer token if not already present
		if user.RealtimeToken == "" && user.ID != "" {
			user.RealtimeToken = user.client.ntf.GenerateToken(user.ID)
		}

		return user, true, nil
	}

	if len(keys) > 1 {
		return User{}, false, fmt.Errorf("%s is duplicated", field)
	}

	return User{}, false, nil
}

func (c *Client) mergeContacts(a, b []*datastore.Key) []*datastore.Key {
	var all []*datastore.Key
	all = append(all, a...)
	all = append(all, b...)

	var merged []*datastore.Key
	seen := make(map[string]struct{})

	for i := range all {
		keyString := all[i].String()

		if _, isSeen := seen[keyString]; isSeen {
			continue
		}

		seen[keyString] = struct{}{}
		merged = append(merged, all[i])
	}

	return merged
}

func (c *Client) reassignContacts(ctx context.Context, tx *datastore.Transaction, oldUser, newUser *User) error {
	var users []*User
	q := datastore.NewQuery("User").Filter("ContactKeys =", oldUser.Key)
	keys, err := c.db.GetAll(ctx, q, &users)
	if err != nil {
		return err
	}

	for i := range users {
		users[i].ContactKeys = swapKeys(users[i].ContactKeys, oldUser.Key, newUser.Key)
	}

	_, err = tx.PutMulti(keys, users)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) reassignMessageUsers(ctx context.Context, tx *datastore.Transaction, old, newUser *User) error {
	userMessages, err := c.GetUnhydratedMessagesByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of messages and save keys to oldUserMessageKeys slice
	userMessageKeys := make([]*datastore.Key, len(userMessages))
	for i := range userMessages {
		userMessages[i].UserKey = newUser.Key
		userMessages[i].Reads = swapReadUserKeys(userMessages[i].Reads, old.Key, newUser.Key)
		userMessageKeys[i] = userMessages[i].Key
	}

	// Save the messages
	_, err = tx.PutMulti(userMessageKeys, userMessages)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) reassignThreadUsers(ctx context.Context, tx *datastore.Transaction, old, newUser *User) error {
	userThreads, err := c.GetUnhydratedThreadsByUser(ctx, old, &Pagination{Size: -1})
	if err != nil {
		return err
	}

	// Reassign ownership of threads and save keys to oldUserThreadKeys slice
	userThreadKeys := make([]*datastore.Key, len(userThreads))
	for i := range userThreads {
		userThreads[i].UserKeys = swapKeys(userThreads[i].UserKeys, old.Key, newUser.Key)
		userThreads[i].Reads = swapReadUserKeys(userThreads[i].Reads, old.Key, newUser.Key)

		if userThreads[i].OwnerKey.Equal(old.Key) {
			userThreads[i].OwnerKey = newUser.Key
		}

		userThreadKeys[i] = userThreads[i].Key
	}

	// Save the threads
	_, err = tx.PutMulti(userThreadKeys, userThreads)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) reassignEventUsers(ctx context.Context, tx *datastore.Transaction, old, newUser *User) error {
	userEvents, err := c.GetUnhydratedEventsByUser(ctx, old, &Pagination{Size: -1})
	if err != nil {
		return err
	}

	// Reassign ownership of events and save keys to userEvetKeys slice
	userEventKeys := make([]*datastore.Key, len(userEvents))
	for i := range userEvents {
		userEvents[i].UserKeys = swapKeys(userEvents[i].UserKeys, old.Key, newUser.Key)
		userEvents[i].RSVPKeys = swapKeys(userEvents[i].RSVPKeys, old.Key, newUser.Key)
		userEvents[i].Reads = swapReadUserKeys(userEvents[i].Reads, old.Key, newUser.Key)

		if userEvents[i].OwnerKey.Equal(old.Key) {
			userEvents[i].OwnerKey = newUser.Key
		}

		userEventKeys[i] = userEvents[i].Key
	}

	// Save the events
	_, err = tx.PutMulti(userEventKeys, userEvents)
	if err != nil {
		return err
	}

	return nil
}

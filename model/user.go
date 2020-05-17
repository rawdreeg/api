package model

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/crypto/bcrypt"

	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/random"
	"github.com/hiconvo/api/valid"
)

type User struct {
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

type UserPartial struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
	Avatar    string `json:"avatar"`
}

func MapUserToUserPartial(u *User) *UserPartial {
	// If the user does not have any name info, show the part of their email
	// before the "@"
	var fullName string
	if u.FirstName == "" && u.LastName == "" && u.FullName == "" {
		fullName = strings.Split(u.Email, "@")[0]
	} else {
		fullName = u.FullName
	}

	return &UserPartial{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		FullName:  fullName,
		Avatar:    u.Avatar,
	}
}

type UserStore interface {
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, bool, error)
	GetUserByToken(ctx context.Context, token string) (*User, bool, error)
	GetUserByOAuthID(ctx context.Context, oauthtoken, provider string) (*User, bool, error)
	// GetUsersByThread(ctx context.Context, t *Thread) ([]*User, error)
	GetOrCreateUserByEmail(ctx context.Context, email string) (*User, bool, error)
	Commit(ctx context.Context, u *User) error
}

func NewIncompleteUser(emailAddress string) (*User, error) {
	email, err := valid.Email(emailAddress)
	if err != nil {
		return nil, errors.E(errors.Op("model.NewIncompleteUser()"), err)
	}

	user := User{
		Key:       datastore.IncompleteKey("User", nil),
		Email:     email,
		FirstName: strings.Split(email, "@")[0],
		Token:     random.Token(),
		Verified:  false,
		CreatedAt: time.Now(),
	}

	return &user, nil
}

func NewUserWithPassword(emailAddress, firstName, lastName, password string) (*User, error) {
	email, err := valid.Email(emailAddress)
	if err != nil {
		return nil, errors.E(errors.Op("model.NewIncompleteUser()"), err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return nil, errors.E(errors.Opf("model.NewUserWithPassword(%s)", emailAddress), err)
	}

	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           email,
		FirstName:       firstName,
		LastName:        lastName,
		FullName:        "",
		PasswordDigest:  string(hash),
		Token:           random.Token(),
		OAuthGoogleID:   "",
		OAuthFacebookID: "",
		Verified:        false,
		CreatedAt:       time.Now(),
	}

	return &user, nil
}

func NewUserWithOAuth(emailAddress, firstName, lastName, avatar, oAuthProvider, oAuthToken string) (*User, error) {
	var (
		op         = errors.Op("model.NewUserWithOAuth()")
		googleID   string
		facebookID string
	)

	switch oAuthProvider {
	case "google":
		googleID = oAuthToken
	case "facebook":
		facebookID = oAuthToken
	default:
		return nil, errors.E(op, errors.Errorf("%q is not a valid oAuthProvider", oAuthProvider))
	}

	email, err := valid.Email(emailAddress)
	if err != nil {
		return nil, errors.E(errors.Op("model.NewIncompleteUser()"), err)
	}

	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           email,
		Emails:          []string{email},
		FirstName:       firstName,
		LastName:        lastName,
		FullName:        "",
		Avatar:          avatar,
		PasswordDigest:  "",
		Token:           random.Token(),
		OAuthGoogleID:   googleID,
		OAuthFacebookID: facebookID,
		Verified:        true,
		CreatedAt:       time.Now(),
	}

	return &user, nil
}

func (u *User) LoadKey(k *datastore.Key) error {
	u.Key = k

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

func (u *User) GetPasswordResetMagicLink(m magic.Client) string {
	return m.NewLink(u.Key, u.PasswordDigest, "reset")
}

func (u *User) VerifyPasswordResetMagicLink(m magic.Client, id, ts, sig string) error {
	return m.Verify(id, ts, u.PasswordDigest, sig)
}

func (u *User) GetVerifyEmailMagicLink(m magic.Client, email string) string {
	salt := email + strconv.FormatBool(u.HasEmail(email))
	return m.NewLink(u.Key, salt, "verify/"+email)
}

func (u *User) HasEmail(email string) bool {
	email = strings.ToLower(email)

	for i := range u.Emails {
		if u.Emails[i] == email {
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
	email = strings.ToLower(email)

	if !u.HasEmail(email) {
		return nil
	}

	if u.Email == email {
		return errors.E(errors.Opf("models.RemoveEmail(%q)", email), http.StatusBadRequest, map[string]string{
			"message": "You cannot remove your primary email"})
	}

	for i := range u.Emails {
		if u.Emails[i] == email {
			u.Emails[i] = u.Emails[len(u.Emails)-1]
			u.Emails = u.Emails[:len(u.Emails)-1]

			break
		}
	}

	return nil
}

func (u *User) MakeEmailPrimary(email string) error {
	if !u.HasEmail(email) {
		return errors.E(errors.Opf("models.MakeEmailPrimary(%q)", email), http.StatusBadRequest, map[string]string{
			"message": "You cannot make an unverified email primary"})
	}

	u.Email = strings.ToLower(email)
	u.Verified = true

	return nil
}

func (u *User) IsRegistered() bool {
	return (u.IsGoogleLinked || u.IsFacebookLinked || u.IsPasswordSet) && u.Verified
}

func (u *User) DeriveProperties() {
	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(u.FirstName)
		u.LastName = strings.Title(u.LastName)
	}

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

func (u *User) MergeWith(ctx context.Context, oldUser *User) error {
	if u.Key.Incomplete() {
		return errors.Str("models.MergeWith: user's key is incomplete")
	}

	if oldUser.Key.Incomplete() {
		return errors.Str("models.MergeWith: oldUser's key is incomplete")
	}

	return nil
}

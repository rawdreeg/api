package model

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/errors"
)

type Thread struct {
	Key           *datastore.Key   `json:"-"        datastore:"__key__"`
	ID            string           `json:"id"       datastore:"-"`
	OwnerKey      *datastore.Key   `json:"-"`
	Owner         *UserPartial     `json:"owner"    datastore:"-"`
	UserKeys      []*datastore.Key `json:"-"`
	Users         []*User          `json:"-"        datastore:"-"`
	UserPartials  []*UserPartial   `json:"users"    datastore:"-"`
	Subject       string           `json:"subject"  datastore:",noindex"`
	Preview       *Message         `json:"preview"  datastore:",noindex"`
	UserReads     []*UserPartial   `json:"reads"    datastore:"-"`
	Reads         []*Read          `json:"-"        datastore:",noindex"`
	CreatedAt     time.Time        `json:"-"`
	ResponseCount int              `json:"responseCount" datastore:",noindex"`
}

type ThreadStore interface {
	GetThreadByID(ctx context.Context, id string) (*Thread, error)
	GetThreadByInt64ID(ctx context.Context, id int64) (*Thread, error)
	GetUnhydratedThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error)
	GetThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error)
	Commit(ctx context.Context, t *Thread) error
	Delete(ctx context.Context, t *Thread) error
}

func NewThread(subject string, owner *User, users []*User) (*Thread, error) {
	if len(users) > 11 {
		return nil, errors.E(errors.Op("NewThread"), http.StatusBadRequest, map[string]string{
			"message": "Convos have a maximum of 11 members",
		})
	}

	// Get all of the users' keys, remove duplicates, and check whether
	// the owner was included in the users slice
	userKeys := make([]*datastore.Key, 0)
	seen := make(map[string]struct{})
	hasOwner := false
	for _, u := range users {
		if _, alreadySeen := seen[u.ID]; alreadySeen {
			continue
		}
		seen[u.ID] = struct{}{}
		if u.Key.Equal(owner.Key) {
			hasOwner = true
		}
		userKeys = append(userKeys, u.Key)
	}

	// Add the owner to the users if not already present
	if !hasOwner {
		userKeys = append(userKeys, owner.Key)
		users = append(users, owner)
	}

	// If a subject wasn't given, create one that is a list of the participants'
	// names.
	if subject == "" {
		if len(users) == 1 {
			subject = owner.FirstName + "'s Private Convo"
		} else {
			for i, u := range users {
				if i == len(users)-1 {
					subject += "and " + u.FirstName
				} else if i == len(users)-2 {
					subject += u.FirstName + " "
				} else {
					subject += u.FirstName + ", "
				}
			}
		}
	}

	return &Thread{
		Key:          datastore.IncompleteKey("Thread", nil),
		OwnerKey:     owner.Key,
		Owner:        MapUserToUserPartial(owner),
		UserKeys:     userKeys,
		Users:        users,
		UserPartials: MapUsersToUserPartials(users),
		Subject:      subject,
	}, nil
}

func (t *Thread) LoadKey(k *datastore.Key) error {
	t.Key = k

	// Add URL safe key
	t.ID = k.Encode()

	return nil
}

func (t *Thread) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(t)
}

func (t *Thread) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(t, ps); err != nil {
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if mismatch.FieldName != "Preview" {
				return err
			}
		} else {
			return err
		}
	}

	for _, p := range ps {
		if p.Name == "Preview" {
			preview, ok := p.Value.(*Preview)
			if ok {
				t.Preview = &Message{
					Body:      preview.Body,
					User:      preview.Sender,
					Timestamp: preview.Timestamp,
				}
			}
		}
	}

	return nil
}

func (t *Thread) GetReads() []*Read {
	return t.Reads
}

func (t *Thread) SetReads(newReads []*Read) {
	t.Reads = newReads
}

func (t *Thread) GetKey() *datastore.Key {
	return t.Key
}

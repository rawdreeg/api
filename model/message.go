package model

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"

	og "github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/errors"
)

type Message struct {
	Key       *datastore.Key `json:"-"        datastore:"__key__"`
	ID        string         `json:"id"       datastore:"-"`
	UserKey   *datastore.Key `json:"-"`
	User      *UserPartial   `json:"user"     datastore:"-"`
	ParentKey *datastore.Key `json:"-"`
	ParentID  string         `json:"parentId" datastore:"-"`
	Body      string         `json:"body"     datastore:",noindex"`
	Timestamp time.Time      `json:"timestamp"`
	Reads     []*Read        `json:"-"        datastore:",noindex"`
	PhotoKeys []string       `json:"-"`
	Photos    []string       `json:"photos"   datastore:"-"`
	Link      *og.LinkData   `json:"link"     datastore:",noindex"`
}

type MessageStore interface {
	GetMessageByID(ctx context.Context, id string) (*Message, error)
	GetMessagesByKey(ctx context.Context, k *datastore.Key) ([]*Message, error)
	GetMessagesByThread(ctx context.Context, t *Thread) ([]*Message, error)
	// GetMessagesByEvent(ctx context.Context, t *Event) ([]*Message, error)
	GetUnhydratedMessagesByUser(ctx context.Context, u *User, p *Pagination) ([]*Message, error)
	Commit(ctx context.Context, t *Message) error
	CommitMulti(ctx context.Context, messages []*Message) error
	Delete(ctx context.Context, t *Message) error
}

func (m *Message) LoadKey(k *datastore.Key) error {
	m.Key = k

	// Add URL safe key
	if k != nil {
		m.ID = k.Encode()
	}

	return nil
}

func (m *Message) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(m)
}

func (m *Message) Load(ps []datastore.Property) error {
	op := errors.Op("message.Load")

	if err := datastore.LoadStruct(m, ps); err != nil {
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if mismatch.FieldName != "ThreadKey" {
				return errors.E(op, err)
			}
		} else {
			return errors.E(op, err)
		}
	}

	for _, p := range ps {
		if p.Name == "ThreadKey" || p.Name == "ParentKey" {
			k, ok := p.Value.(*datastore.Key)
			if !ok {
				return errors.E(op, errors.Errorf("could not load parent key into message='%v'", m.ID))
			}
			m.ParentKey = k
			m.ParentID = k.Encode()
		}

		// Convert photoKeys into full URLs
		if p.Name == "PhotoKeys" {
			photoKeys, ok := p.Value.([]interface{})
			if ok {
				photos := make([]string, len(photoKeys))
				for i := range photoKeys {
					photoKey, ok := photoKeys[i].(string)
					if ok {
						photos[i] = photoKey
					}
				}

				m.Photos = photos
			}
		}
	}

	return nil
}

func (m *Message) GetReads() []*Read {
	return m.Reads
}

func (m *Message) SetReads(newReads []*Read) {
	m.Reads = newReads
}

func (m *Message) HasPhoto() bool {
	return len(m.PhotoKeys) > 0
}

func (m *Message) HasLink() bool {
	return m.Link != nil
}

func (m *Message) OwnerIs(u *User) bool {
	return m.UserKey.Equal(u.Key)
}

func (m *Message) HasPhotoKey(key string) bool {
	for i := range m.PhotoKeys {
		if m.PhotoKeys[i] == key {
			return true
		}
	}

	return false
}

func MarkMessagesAsRead(
	ctx context.Context,
	s MessageStore,
	r Readable,
	u *User,
	parentKey *datastore.Key,
) error {
	op := errors.Op("model.MarkMessagesAsRead")

	messages, err := s.GetMessagesByKey(ctx, parentKey)
	if err != nil {
		return errors.E(op, err)
	}

	for i := range messages {
		MarkAsRead(messages[i], u.Key)
	}

	err = s.CommitMulti(ctx, messages)
	if err != nil {
		return errors.E(op, err)
	}

	return nil
}

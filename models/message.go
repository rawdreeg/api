package models

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/models/read"
	og "github.com/hiconvo/api/utils/opengraph"
)

type Message struct {
	client *Client `datastore:"-"`

	Key       *datastore.Key `json:"-"        datastore:"__key__"`
	ID        string         `json:"id"       datastore:"-"`
	UserKey   *datastore.Key `json:"-"`
	User      *UserPartial   `json:"user"     datastore:"-"`
	ParentKey *datastore.Key `json:"-"`
	ParentID  string         `json:"parentId" datastore:"-"`
	Body      string         `json:"body"     datastore:",noindex"`
	Timestamp time.Time      `json:"timestamp"`
	Reads     []*read.Read   `json:"-"        datastore:",noindex"`
	PhotoKeys []string       `json:"-"`
	Photos    []string       `json:"photos"   datastore:"-"`
	Link      *og.LinkData   `json:"link"     datastore:",noindex"`
}

func (c *Client) NewThreadMessage(u *User, t *Thread, body, photoKey string, link og.LinkData) (Message, error) {
	ts := time.Now()

	linkPtr := &link
	if link.URL == "" {
		linkPtr = nil
	}

	message := Message{
		client: c,

		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToUserPartial(u),
		ParentKey: t.Key,
		ParentID:  t.ID,
		Body:      removeLink(body, linkPtr),
		Timestamp: ts,
		Link:      linkPtr,
	}

	if photoKey != "" {
		message.PhotoKeys = []string{photoKey}
		message.Photos = []string{c.storage.GetPhotoURLFromKey(photoKey)}
	}

	if t.Preview == nil {
		t.Preview = &message
	}

	t.IncRespCount()

	read.ClearReads(t)
	read.MarkAsRead(t, u.Key)

	return message, nil
}

func (c *Client) NewEventMessage(u *User, e *Event, body, photoKey string) (Message, error) {
	ts := time.Now()

	message := Message{
		client: c,

		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToUserPartial(u),
		ParentKey: e.Key,
		ParentID:  e.ID,
		Body:      body,
		Timestamp: ts,
	}

	if photoKey != "" {
		message.PhotoKeys = []string{photoKey}
		message.Photos = []string{c.storage.GetPhotoURLFromKey(photoKey)}
	}

	read.ClearReads(e)
	read.MarkAsRead(e, u.Key)

	return message, nil
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
	}

	return nil
}

func (m *Message) GetReads() []*read.Read {
	return m.Reads
}

func (m *Message) SetReads(newReads []*read.Read) {
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

func (m *Message) setPhotoURLs() {
	if len(m.PhotoKeys) > 0 {
		photos := make([]string, len(m.PhotoKeys))

		for i := range m.PhotoKeys {
			photos[i] = m.client.storage.GetPhotoURLFromKey(m.PhotoKeys[i])
		}

		m.Photos = photos
	}
}

// DeletePhoto deletes the given photo by key. In order to handle
// concurrent requests, or cases where photo deletion succeeds but
// updating the message fails, etc., it does not return an if the
// photo has already been deleted.
func (m *Message) DeletePhoto(ctx context.Context, key string) error {
	if m.HasPhotoKey(key) {
		for i := range m.PhotoKeys {
			if m.PhotoKeys[i] == key {
				m.PhotoKeys[i] = m.PhotoKeys[len(m.PhotoKeys)-1]
				m.PhotoKeys = m.PhotoKeys[:len(m.PhotoKeys)-1]
				break
			}
		}

		for i := range m.Photos {
			if strings.HasSuffix(m.Photos[i], key) {
				m.Photos[i] = m.Photos[len(m.Photos)-1]
				m.Photos = m.Photos[:len(m.Photos)-1]
				break
			}
		}

		if err := m.client.storage.DeletePhoto(ctx, key); err != nil {
			log.Alarm(errors.E(errors.Op("models.DeletePhoto"), err))
		}
	}

	return nil
}

func (m *Message) Commit(ctx context.Context) error {
	key, err := m.client.db.Put(ctx, m.Key, m)
	if err != nil {
		return err
	}

	m.ID = key.Encode()
	m.Key = key

	return nil
}

func (m *Message) Delete(ctx context.Context) error {
	if err := m.client.db.Delete(ctx, m.Key); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetMessagesByThread(ctx context.Context, t *Thread) ([]*Message, error) {
	return c.GetMessagesByKey(ctx, t.Key)
}

func (c *Client) GetMessagesByEvent(ctx context.Context, e *Event) ([]*Message, error) {
	return c.GetMessagesByKey(ctx, e.Key)
}

func (c *Client) GetMessagesByKey(ctx context.Context, k *datastore.Key) ([]*Message, error) {
	var messages []*Message

	q := datastore.NewQuery("Message").Filter("ParentKey =", k)

	if _, err := c.db.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	userKeys := make([]*datastore.Key, len(messages))
	for i := range messages {
		userKeys[i] = messages[i].UserKey
	}
	users := make([]*User, len(userKeys))
	if err := c.db.GetMulti(ctx, userKeys, users); err != nil {
		return messages, err
	}

	for i := range messages {
		messages[i].client = c
		messages[i].User = MapUserToUserPartial(users[i])
		messages[i].setPhotoURLs()
	}

	// TODO: Get Query#Order to work above.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

func (c *Client) GetUnhydratedMessagesByUser(ctx context.Context, u *User) ([]*Message, error) {
	var messages []*Message
	q := datastore.NewQuery("Message").Filter("UserKey =", u.Key)
	if _, err := c.db.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	for i := range messages {
		messages[i].client = c
		messages[i].setPhotoURLs()
	}

	return messages, nil
}

func (c *Client) GetMessageByID(ctx context.Context, id string) (Message, error) {
	var op errors.Op = "models.GetMessageByID"
	var message Message

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return message, errors.E(op, err)
	}

	err = c.db.Get(ctx, key, &message)
	if err != nil {
		return message, errors.E(op, err)
	}

	message.client = c
	message.setPhotoURLs()

	return message, nil
}

func removeLink(body string, linkPtr *og.LinkData) string {
	if linkPtr == nil {
		return body
	}

	// If this is a markdown formatted link, leave it. Otherwise, remove the link.
	// This isn't a perfect test, but it gets the job done and I'm lazy.
	if strings.Contains(body, fmt.Sprintf("[%s]", linkPtr.URL)) {
		return body
	}

	return strings.Replace(body, linkPtr.URL, "", 1)
}

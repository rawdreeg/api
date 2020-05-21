package model

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"

	og "github.com/hiconvo/api/clients/opengraph"
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
	Delete(ctx context.Context, t *Message) error
}

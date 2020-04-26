package models

import (
	"context"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/models/read"
)

type DigestItem struct {
	ParentID *datastore.Key
	Name     string
	Messages []*Message
}

type Digestable interface {
	GetKey() *datastore.Key
	GetName() string
	GetMessages(context.Context) ([]*Message, error)
}

var _digestErr = errors.E(errors.Op("digest"))

type Digestor struct {
	user *User
}

func NewDigestor(u *User) *Digestor {
	return &Digestor{u}
}

func (c *Digestor) Do(ctx context.Context) error {
	events, err := c.user.client.GetEventsByUser(ctx, c.user, &Pagination{})
	if err != nil {
		return err
	}

	threads, err := c.user.client.GetThreadsByUser(ctx, c.user, &Pagination{})
	if err != nil {
		return err
	}

	// Convert the events into Digestables and filter out read items
	var digestables []Digestable
	// Save the upcoming events to a slice at the same time
	var upcoming []*Event
	for i := range events {
		if !read.IsRead(events[i], c.user.Key) {
			digestables = append(digestables, events[i])
		}

		if events[i].IsUpcoming() {
			upcoming = append(upcoming, events[i])
		}
	}

	for i := range threads {
		if !read.IsRead(threads[i], c.user.Key) {
			digestables = append(digestables, threads[i])
		}
	}

	digestList, err := c.generateDigestList(ctx, digestables)
	if err != nil {
		return err
	}

	if len(digestList) > 0 || len(upcoming) > 0 {
		if err := c.user.client.mail.SendDigest(digestList, upcoming, c.user); err != nil {
			return err
		}

		if err := c.markDigestedMessagesAsRead(ctx, digestList); err != nil {
			return err
		}
	}

	return nil
}

func (c *Digestor) generateDigestList(ctx context.Context, digestables []Digestable) ([]DigestItem, error) {
	var digest []DigestItem
	for i := range digestables {
		item, err := c.generateDigestItem(ctx, digestables[i])
		if err != nil {
			if err == _digestErr {
				continue
			}

			return digest, errors.E(errors.Opf("models.GenerateDigestList(userID=%s)", c.user.ID), err)
		}

		digest = append(digest, item)
	}

	return digest, nil
}

func (c *Digestor) generateDigestItem(ctx context.Context, d Digestable) (DigestItem, error) {
	messages, err := d.GetMessages(ctx)
	if err != nil {
		return DigestItem{}, err
	}

	var unread []*Message
	for j := range messages {
		if !read.IsRead(messages[j], c.user.Key) {
			unread = append(unread, messages[j])
		}
	}

	if len(unread) > 0 {
		return DigestItem{
			ParentID: d.GetKey(),
			Name:     d.GetName(),
			Messages: unread,
		}, nil
	}

	return DigestItem{}, _digestErr
}

func (c *Digestor) markDigestedMessagesAsRead(ctx context.Context, digestList []DigestItem) error {
	var messages []*Message
	var keys []*datastore.Key
	for i := range digestList {
		for j := range digestList[i].Messages {
			read.MarkAsRead(digestList[i].Messages[j], c.user.Key)
			messages = append(messages, digestList[i].Messages[j])
			keys = append(keys, digestList[i].Messages[j].Key)
		}
	}

	_, err := c.user.client.db.PutMulti(ctx, keys, messages)
	if err != nil {
		return err
	}

	return nil
}

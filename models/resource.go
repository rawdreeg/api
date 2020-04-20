package models

import (
	"context"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/notifications"
	"github.com/hiconvo/api/queue"
	"github.com/hiconvo/api/search"
	"github.com/hiconvo/api/utils/secrets"
)

type Client struct {
	db     db.Client            `datastore:"-"`
	ntf    notifications.Client `datastore:"-"`
	search search.Client        `datastore:"-"`
	mail   mail.Client          `datastore:"-"`
	queue  queue.Client         `datastore:"-"`

	supportUser    *User
	welcomeMessage string
}

func NewClient(dc db.Client, nc notifications.Client, sc search.Client, mc mail.Client, qc queue.Client) *Client {
	c := &Client{
		db:     dc,
		ntf:    nc,
		search: sc,
		mail:   mc,
		queue:  qc,
	}
	c.initSupportUser()
	c.welcomeMessage = readStringFromFile("welcome.md")
	return c
}

func (c *Client) initSupportUser() {
	op := errors.Op("models.Client.initSupportUser")
	supportPassword := secrets.Get("SUPPORT_PASSWORD", "support")
	ctx := context.Background()

	u, found, err := c.GetUserByEmail(ctx, "support@convo.events")
	if err != nil {
		panic(err)
	}

	if !found {
		u, err = c.NewUserWithPassword(
			"support@convo.events", "Convo Support", "", supportPassword)
		if err != nil {
			panic(errors.E(op, err))
		}

		err = u.Commit(ctx)
		if err != nil {
			panic(errors.E(op, err))
		}

		log.Print("models.Client.initSupportUser: Created new support user")
	}

	c.supportUser = &u
}

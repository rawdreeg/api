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
	"github.com/hiconvo/api/storage"
	"github.com/hiconvo/api/utils/magic"
)

type Client struct {
	db      db.Client
	ntf     notifications.Client
	search  search.Client
	mail    *MailClient
	queue   queue.Client
	storage *storage.Client
	magic   magic.Client

	supportUser    *User
	welcomeMessage string
}

type Config struct {
	DB              db.Client
	Queue           queue.Client
	Ntf             notifications.Client
	Storage         *storage.Client
	Mail            mail.Client
	Magic           magic.Client
	Search          search.Client
	SupportPassword string
}

func NewClient(cfg *Config) *Client {
	c := &Client{
		db:      cfg.DB,
		ntf:     cfg.Ntf,
		search:  cfg.Search,
		mail:    NewMailClient(cfg.Mail, cfg.Magic),
		queue:   cfg.Queue,
		storage: cfg.Storage,
		magic:   cfg.Magic,
	}
	c.initSupportUser(cfg.SupportPassword)
	c.welcomeMessage = readStringFromFile("welcome.md")
	return c
}

func (c *Client) initSupportUser(supportPassword string) {
	op := errors.Op("models.Client.initSupportUser")
	ctx := context.Background()

	u, found, err := c.GetUserByEmail(ctx, "support@convo.events")
	if err != nil {
		panic(errors.E(op, err))
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

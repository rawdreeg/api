package db

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

var _ model.UserStore = (*UserStore)(nil)

type UserStore struct {
	DB     db.Client
	Notif  notification.Client
	Search search.Client
}

func (s *UserStore) Commit(ctx context.Context, u *model.User) error {
	op := errors.Opf("UserStore.Commit(%q)", u.Email)

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(strings.TrimSpace(u.FirstName))
		u.LastName = strings.Title(strings.TrimSpace(u.LastName))
	}

	key, err := s.DB.Put(ctx, u.Key, u)
	if err != nil {
		return errors.E(op, err)
	}

	u.ID = key.Encode()
	u.Key = key

	// We have to do this after the user has been saved because we need the
	// ID, which isn't available until the user is in the database
	u.RealtimeToken = s.Notif.GenerateToken(u.ID)
	u.DeriveProperties()

	s.createOrUpdateSearchIndex(ctx, u)

	return nil
}

func (s *UserStore) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	op := errors.Opf("UserStore.GetUserByID(id=%s)", id)

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return nil, errors.E(op, err, http.StatusNotFound)
	}

	u := new(model.User)
	if err := s.DB.Get(ctx, key, u); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, errors.E(op, http.StatusNotFound, err)
		}

		return nil, errors.E(op, err)
	}

	return u, nil
}

func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, bool, error) {
	op := errors.Opf("UserStore.GetUserByEmail(email=%q)", email)

	email, err := valid.Email(email)
	if err != nil {
		return nil, false, errors.E(op, err)
	}

	u, found, err := s.getUserByField(ctx, "Email", email)
	if !found && err == nil {
		return s.getUserByField(ctx, "Emails", email)
	}

	return u, found, err
}

func (s *UserStore) GetUserByToken(ctx context.Context, token string) (*model.User, bool, error) {
	return s.getUserByField(ctx, "Token", token)
}

func (s *UserStore) GetUserByOAuthID(ctx context.Context, oAuthToken, provider string) (*model.User, bool, error) {
	if provider == "google" {
		return s.getUserByField(ctx, "OAuthGoogleID", oAuthToken)
	}

	return s.getUserByField(ctx, "OAuthFacebookID", oAuthToken)
}

// func (s *UserStore) GetUsersByThread(ctx context.Context, t *model.Thread) ([]*model.User, error) {
// 	var userKeys []*datastore.Key
// 	copy(userKeys, t.UserKeys)
// 	userKeys = append(userKeys, t.OwnerKey)

// 	users := make([]*model.User, len(userKeys))
// 	if err := s.DB.GetMulti(ctx, userKeys, users); err != nil {
// 		return users, err
// 	}

// 	return users, nil
// }

func (s *UserStore) GetOrCreateUserByEmail(ctx context.Context, email string) (*model.User, bool, error) {
	u, found, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, false, err
	} else if found {
		return u, false, nil
	}

	u, err = model.NewIncompleteUser(email)
	if err != nil {
		return nil, false, err
	}

	return u, true, nil
}

func (s *UserStore) getUserByField(ctx context.Context, field, value string) (*model.User, bool, error) {
	var (
		op    = errors.Opf("UserStore.getUserByField(field=%q, value=%q)", field, value)
		users []model.User
	)

	q := datastore.NewQuery("User").Filter(fmt.Sprintf("%s =", field), value)

	keys, err := s.DB.GetAll(ctx, q, &users)
	if err != nil {
		return nil, false, errors.E(op, err)
	}

	if len(keys) == 1 {
		user := users[0]

		// Generate the streamer token if not already present
		if user.RealtimeToken == "" && user.ID != "" {
			user.RealtimeToken = s.Notif.GenerateToken(user.ID)
		}

		return &user, true, nil
	}

	if len(keys) > 1 {
		return nil, false, errors.E(op, errors.Errorf("field=%q value=%q is duplicated", field, value))
	}

	return nil, false, nil
}

func (s *UserStore) createOrUpdateSearchIndex(ctx context.Context, u *model.User) {
	if u.IsRegistered() {
		_, upsertErr := s.Search.Update().
			Index("users").
			Id(u.ID).
			DocAsUpsert(true).
			Doc(model.MapUserToUserPartial(u)).
			Do(ctx)
		if upsertErr != nil {
			log.Printf("Failed to index user in elasticsearch: %v", upsertErr)
		}
	}
}
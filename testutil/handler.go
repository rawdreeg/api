package testutil

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/icrowley/fake"

	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	sender "github.com/hiconvo/api/clients/mail"
	"github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/oauth"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/handler"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/template"
)

func Handler(dbClient dbc.Client, searchClient search.Client) http.Handler {
	return handler.New(&handler.Config{
		UserStore: &db.UserStore{DB: dbClient, Notif: notification.NewLogger(), S: searchClient},
		Mail:      mail.New(sender.NewLogger(), template.NewClient()),
		Magic:     magic.NewClient(""),
		Storage:   storage.NewClient("", ""),
		OAuth:     oauth.NewClient(""),
	})
}

func NewUser(ctx context.Context, t *testing.T, dbClient dbc.Client, searchClient search.Client) (*model.User, string) {
	t.Helper()

	email := fake.EmailAddress()
	pw := fake.SimplePassword()

	u, err := model.NewUserWithPassword(
		email,
		fake.FirstName(),
		fake.LastName(),
		pw)
	if err != nil {
		t.Fatal(err)
	}

	u.Verified = true

	s := NewUserStore(ctx, t, dbClient, searchClient)

	err = s.Commit(ctx, u)
	if err != nil {
		t.Fatal(err)
	}

	return u, pw
}

func NewIncompleteUser(ctx context.Context, t *testing.T, dbClient dbc.Client, searchClient search.Client) *model.User {
	t.Helper()

	u, err := model.NewIncompleteUser(fake.EmailAddress())
	if err != nil {
		t.Fatal(err)
	}

	s := NewUserStore(ctx, t, dbClient, searchClient)

	err = s.Commit(ctx, u)
	if err != nil {
		t.Fatal(err)
	}

	return u
}

func NewNotifClient(t *testing.T) notification.Client {
	t.Helper()
	return notification.NewLogger()
}

func NewUserStore(ctx context.Context, t *testing.T, dbClient dbc.Client, searchClient search.Client) model.UserStore {
	t.Helper()
	return &db.UserStore{DB: dbClient, Notif: notification.NewLogger(), S: searchClient}
}

func NewSearchClient() search.Client {
	return search.NewClient("elasticsearch")
}

func NewDBClient(ctx context.Context) dbc.Client {
	return dbc.NewClient(ctx, "local-convo-api")
}

func ClearDB(ctx context.Context, client dbc.Client) {
	for _, tp := range []string{"User", "Thread", "Event", "Message"} {
		q := datastore.NewQuery(tp).KeysOnly()

		keys, err := client.GetAll(ctx, q, nil)
		if err != nil {
			panic(err)
		}

		err = client.DeleteMulti(ctx, keys)
		if err != nil {
			panic(err)
		}
	}
}

func GetMagicLinkParts(link string) (string, string, string) {
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]
	return kenc, b64ts, sig
}

func GetAuthHeader(token string) map[string]string {
	return map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)}
}

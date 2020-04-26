package router_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/handlers"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/notifications"
	"github.com/hiconvo/api/queue"
	"github.com/hiconvo/api/search"
	"github.com/hiconvo/api/storage"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/oauth"
	og "github.com/hiconvo/api/utils/opengraph"
	"github.com/hiconvo/api/utils/places"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/secrets"
	"github.com/hiconvo/api/utils/thelpers"
)

var (
	tc           context.Context
	th           http.Handler
	tclient      *datastore.Client
	modelsClient *models.Client
	magicClient  magic.Client
)

func TestMain(m *testing.M) {
	os.Chdir("..")

	ctx := thelpers.CreateTestContext()
	client := thelpers.CreateTestDatastoreClient(ctx)
	thelpers.ClearDatastore(ctx, client)

	dbClient := db.NewClient(ctx, "local-convo-api")
	secretsClient := secrets.NewClient(ctx, dbClient)

	// Set globals to be used by tests below
	tc = ctx

	handlerCfg := &handlers.Config{
		DB:              dbClient,
		Queue:           queue.NewLogger(),
		Places:          places.NewLogger(),
		Ntf:             notifications.NewLogger(),
		Storage:         storage.NewClient("", ""),
		Mail:            mail.NewLogger(),
		OAuthClient:     oauth.NewClient(""),
		Magic:           magic.NewClient("testing"),
		Search:          search.NewClient(secretsClient.Get("ELASTICSEARCH_HOST", "elasticsearch")),
		SupportPassword: "supportPassword",
	}
	th = handlers.New(handlerCfg)

	modelsClient = handlerCfg.ModelsClient
	magicClient = handlerCfg.Magic

	tclient = client

	result := m.Run()

	thelpers.ClearDatastore(ctx, client)

	os.Exit(result)
}

func Test404(t *testing.T) {
	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "GET", fmt.Sprintf("/%s", random.String(8)), nil, nil)
	thelpers.AssertStatusCodeEqual(t, rr, http.StatusNotFound)
}

func createTestUser(t *testing.T) (models.User, string) {
	password := random.String(20)
	u, err := modelsClient.NewUserWithPassword(
		strings.ToLower(fmt.Sprintf("%s@test.com", random.String(20))),
		random.String(20),
		random.String(20),
		password,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Mark the user as verified by default
	u.Verified = true

	// Save the user
	if err := u.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return u, password
}

func createTestThread(t *testing.T, owner *models.User, users []*models.User) models.Thread {
	// Create the thread.
	thread, err := modelsClient.NewThread("test", owner, users)
	if err != nil {
		t.Fatal(err)
	}

	// Save the thread
	if err := thread.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return thread
}

func createTestThreadMessage(t *testing.T, user *models.User, thread *models.Thread) models.Message {
	message, err := modelsClient.NewThreadMessage(user, thread, random.String(50), "", og.LinkData{})
	if err != nil {
		t.Fatal(err)
	}

	// Save the message
	if err := message.Commit(tc); err != nil {
		t.Fatal(err)
	}

	if err := thread.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return message
}

func createTestEvent(t *testing.T, owner *models.User, users, hosts []*models.User) *models.Event {
	// Create the thread.
	event, err := modelsClient.NewEvent(
		"test",
		"locKey",
		"loc",
		"description",
		0.0,
		0.0,
		time.Now().Add(time.Duration(1000000000000000)),
		-7*60*60,
		owner,
		hosts,
		users,
		false)
	if err != nil {
		t.Fatal(err)
	}

	eptr := &event

	// Save the event.
	if err := eptr.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return eptr
}

func createTestEventMessage(t *testing.T, user *models.User, event *models.Event) models.Message {
	message, err := modelsClient.NewEventMessage(user, event, random.String(50), "")
	if err != nil {
		t.Fatal(err)
	}

	// Save the message
	if err := message.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return message
}

func getAuthHeader(token string) map[string]string {
	return map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)}
}

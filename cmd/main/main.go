package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/getsentry/raven-go"

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
	"github.com/hiconvo/api/utils/places"
	"github.com/hiconvo/api/utils/secrets"
)

func main() {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	ctx := context.Background()

	dbClient := db.NewClient(ctx, projectID)
	secretsClient := secrets.NewClient(ctx, dbClient)

	raven.SetDSN(secretsClient.Get("SENTRY_DSN", ""))
	raven.SetRelease(os.Getenv("GAE_VERSION"))

	ntfClient := notifications.NewClient(
		secretsClient.Get("STREAM_API_KEY", "streamKey"),
		secretsClient.Get("STREAM_API_SECRET", "streamSecret"),
		"us-east",
	)
	storageClient := storage.NewClient(
		secretsClient.Get("AVATAR_BUCKET_NAME", ""),
		secretsClient.Get("PHOTO_BUCKET_NAME", ""),
	)
	mailClient := mail.NewClient(secretsClient.Get("SENDGRID_API_KEY", ""))
	oauthClient := oauth.NewClient(secretsClient.Get("GOOGLE_OAUTH_KEY", ""))
	magicClient := magic.NewClient(secretsClient.Get("APP_SECRET", ""))

	var queueClient queue.Client
	if projectID == "local-convo-api" || projectID == "" {
		queueClient = queue.NewLogger()
	} else {
		queueClient = queue.NewClient(ctx, projectID)
	}

	http.Handle("/", handlers.New(&handlers.Config{
		ModelsClient: models.NewClient(
			dbClient,
			ntfClient,
			search.NewClient(secretsClient.Get("ELASTICSEARCH_HOST", "elasticsearch")),
			mailClient,
			queueClient,
			storageClient,
			magicClient,
			secretsClient.Get("SUPPORT_PASSWORD", ""),
		),
		DB:            dbClient,
		PlacesClient:  places.NewClient(secretsClient.Get("GOOGLE_MAPS_API_KEY", "")),
		NtfClient:     ntfClient,
		StorageClient: storageClient,
		MailClient:    mailClient,
		OAuthClient:   oauthClient,
		MagicClient:   magicClient,
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/raven-go"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/handlers"
	"github.com/hiconvo/api/mail"
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
	ctx := context.Background()
	projectID := envget("GOOGLE_CLOUD_PROJECT", "local-convo-api")

	dbClient := db.NewClient(ctx, projectID)
	sc := secrets.NewClient(ctx, dbClient)

	raven.SetDSN(sc.Get("SENTRY_DSN", ""))
	raven.SetRelease(envget("GAE_VERSION", "dev"))

	http.Handle("/", handlers.New(&handlers.Config{
		DB:              dbClient,
		Queue:           getQueueClient(ctx, projectID),
		Places:          places.NewClient(sc.Get("GOOGLE_MAPS_API_KEY", "")),
		Ntf:             notifications.NewClient(sc.Get("STREAM_API_KEY", "streamKey"), sc.Get("STREAM_API_SECRET", "streamSecret"), "us-east"),
		Storage:         storage.NewClient(sc.Get("AVATAR_BUCKET_NAME", ""), sc.Get("PHOTO_BUCKET_NAME", "")),
		Mail:            mail.NewClient(sc.Get("SENDGRID_API_KEY", "")),
		OAuthClient:     oauth.NewClient(sc.Get("GOOGLE_OAUTH_KEY", "")),
		Magic:           magic.NewClient(sc.Get("APP_SECRET", "")),
		Search:          search.NewClient(sc.Get("ELASTICSEARCH_HOST", "elasticsearch")),
		SupportPassword: sc.Get("SUPPORT_PASSWORD", ""),
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	srv := http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		Addr:         fmt.Sprintf(":%s", port),
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(srv.ListenAndServe())
}

func envget(name, fallback string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}

	return fallback
}

func getQueueClient(ctx context.Context, projectID string) queue.Client {
	if projectID == "local-convo-api" || projectID == "" {
		return queue.NewLogger()
	}

	return queue.NewClient(ctx, projectID)
}

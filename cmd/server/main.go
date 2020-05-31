package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/raven-go"

	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	sender "github.com/hiconvo/api/clients/mail"
	"github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/oauth"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/clients/secrets"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/handler"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/template"
)

const (
	readTimeout  = 5
	writeTimeout = 60
)

func main() {
	ctx := context.Background()
	projectID := getenv("GOOGLE_CLOUD_PROJECT", "local-convo-api")
	port := getenv("PORT", "8080")

	dbClient := dbc.NewClient(ctx, projectID)
	defer dbClient.Close()

	sc := secrets.NewClient(ctx, dbClient)

	raven.SetDSN(sc.Get("SENTRY_DSN", ""))
	raven.SetRelease(getenv("GAE_VERSION", "dev"))

	notifClient := notification.NewClient(
		sc.Get("STREAM_API_KEY", "streamKey"),
		sc.Get("STREAM_API_SECRET", "streamSecret"),
		"us-east")
	mailClient := mail.New(
		sender.NewClient(sc.Get("SENDGRID_API_KEY", "")),
		template.NewClient(),
	)
	searchClient := search.NewClient(sc.Get("ELASTICSEARCH_HOST", "elasticsearch"))
	storageClient := storage.NewClient(
		sc.Get("AVATAR_BUCKET_NAME", ""),
		sc.Get("PHOTO_BUCKET_NAME", ""))

	h := handler.New(&handler.Config{
		UserStore:     &db.UserStore{DB: dbClient, Notif: notifClient, S: searchClient},
		ThreadStore:   &db.ThreadStore{DB: dbClient},
		MessageStore:  &db.MessageStore{DB: dbClient},
		TxnMiddleware: dbc.WithTransaction(dbClient),
		Mail:          mailClient,
		Magic:         magic.NewClient(sc.Get("APP_SECRET", "")),
		OAuth:         oauth.NewClient(sc.Get("GOOGLE_AUD", "")),
		Storage:       storageClient,
		Notif:         notifClient,
	})

	srv := http.Server{
		Handler:      h,
		ReadTimeout:  readTimeout * time.Second,
		WriteTimeout: writeTimeout * time.Second,
		Addr:         fmt.Sprintf(":%s", port),
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(srv.ListenAndServe())
}

func getenv(name, fallback string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return fallback
}

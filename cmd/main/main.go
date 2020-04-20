package main

import (
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
	"github.com/hiconvo/api/utils/secrets"
)

func main() {
	raven.SetDSN(secrets.Get("SENTRY_DSN", ""))
	raven.SetRelease(os.Getenv("GAE_VERSION"))

	http.Handle("/", handlers.New(&handlers.Config{
		ModelsClient: models.NewClient(
			db.DefaultClient,
			notifications.DefaultClient,
			search.DefaultClient,
			mail.DefaultClient,
			queue.DefaultClient),
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

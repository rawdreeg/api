package handler

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/handler/user"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
)

type Config struct {
	UserStore model.UserStore
	Mail      *mail.Client
	Magic     magic.Client
}

func New(c *Config) http.Handler {
	router := mux.NewRouter()

	router.Use(middleware.WithJSONRequests)

	router.NotFoundHandler = http.HandlerFunc(notFound)

	router.PathPrefix("/users").Handler(user.NewHandler(&user.Config{
		UserStore: c.UserStore,
		Mail:      c.Mail,
		Magic:     c.Magic,
	}))

	h := middleware.WithCORS(router)
	h = middleware.WithLogging(h)
	h = middleware.WithErrorReporting(h)

	return h
}

func notFound(w http.ResponseWriter, r *http.Request) {
	bjson.WriteJSON(w, map[string]string{"message": "Not found"}, http.StatusNotFound)
}

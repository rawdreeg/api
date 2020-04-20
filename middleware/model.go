package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

type contextKey int

const (
	userKey contextKey = iota
	threadKey
	eventKey
)

// UserFromContext retuns the User object that was added to the context via
// WithUser middleware.
func UserFromContext(ctx context.Context) models.User {
	return ctx.Value(userKey).(models.User)
}

// WithUser adds the authenticated user to the context. If the user cannot be
// found, then a 401 unauthorized reponse is returned.
func WithUser(c *models.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var op errors.Op = "middleware.WithUser"

			if token, ok := GetAuthToken(r.Header); ok {
				ctx := r.Context()
				user, ok, err := c.GetUserByToken(ctx, token)
				if err != nil {
					bjson.HandleError(w, errors.E(op, err))
					return
				}

				if ok {
					next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, userKey, user)))
					return
				}
			}

			bjson.HandleError(w, errors.E(op, http.StatusUnauthorized, errors.Str("NoToken")))
		})
	}
}

// GetAuthToken extracts the Authorization Bearer token from request
// headers if present.
func GetAuthToken(h http.Header) (string, bool) {
	if val := h.Get("Authorization"); val != "" {
		if strings.ToLower(val[:7]) == "bearer " {
			return val[7:], true
		}
	}

	return "", false
}

// EventFromContext retuns the Event object that was added to the context via
// WithEvent middleware.
func EventFromContext(ctx context.Context) models.Event {
	return ctx.Value(eventKey).(models.Event)
}

// WithEvent adds the event indicated in the url to the context. If the event
// cannot be found, then a 404 reponse is returned.
func WithEvent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)
		id := vars["eventID"]

		event, err := models.GetEventByID(ctx, id)
		if err != nil {
			bjson.HandleError(w, errors.E(errors.Op("middleware.WithEvent"), http.StatusNotFound, err))
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, eventKey, event)))
		return
	})
}

// ThreadFromContext retuns the Thread object that was added to the context via
// WithThread middleware.
func ThreadFromContext(ctx context.Context) models.Thread {
	return ctx.Value(threadKey).(models.Thread)
}

// WithThread adds the thread indicated in the url to the context. If the thread
// cannot be found, then a 404 reponse is returned.
func WithThread(c *models.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			vars := mux.Vars(r)
			id := vars["threadID"]

			thread, err := c.GetThreadByID(ctx, id)
			if err != nil {
				bjson.HandleError(w, errors.E(errors.Op("middleware.WithThread"), http.StatusNotFound, err))
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, threadKey, thread)))
			return
		})
	}
}

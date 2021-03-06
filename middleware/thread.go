package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

type threadContextKey string

const threadKey threadContextKey = "thread"

// ThreadFromContext retuns the Thread object that was added to the context via
// WithThread middleware.
func ThreadFromContext(ctx context.Context) models.Thread {
	return ctx.Value(threadKey).(models.Thread)
}

// WithThread adds the thread indicated in the url to the context. If the thread
// cannot be found, then a 404 reponse is returned.
func WithThread(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)
		id := vars["threadID"]

		thread, err := models.GetThreadByID(ctx, id)
		if err != nil {
			bjson.HandleError(w, errors.E(errors.Op("middleware.WithThread"), http.StatusNotFound, err))
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, threadKey, thread)))
		return
	})
}

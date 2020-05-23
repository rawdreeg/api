package thread

import (
	"html"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

type Config struct {
	UserStore   model.UserStore
	ThreadStore model.ThreadStore
	Mail        *mail.Client
	Magic       magic.Client
	Storage     *storage.Client
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.Use(middleware.WithUser(c.UserStore))
	r.HandleFunc("/threads", c.CreateThread).Methods("POST")
	r.HandleFunc("/threads", c.GetThreads).Methods("GET")

	s := r.NewRoute().Subrouter()
	s.Use(middleware.WithThread(c.ThreadStore))
	s.HandleFunc("/threads/{threadID}", c.GetThread).Methods("GET")
	s.HandleFunc("/threads/{threadID}", c.DeleteThread).Methods("DELETE")
	// s.HandleFunc("/threads/{threadID}/messages", c.GetMessagesByThread).Methods("GET")
	// s.HandleFunc("/threads/{threadID}/reads", c.MarkThreadAsRead).Methods("POST")

	// t := r.NewRoute().Subrouter()
	// t.Use(db.WithTransaction())
	// t.HandleFunc("/threads/{threadID}", c.UpdateThread).Methods("PATCH")
	// t.HandleFunc("/threads/{threadID}/users/{userID}", c.AddUserToThread).Methods("POST")
	// t.HandleFunc("/threads/{threadID}/users/{userID}", c.RemoveUserFromThread).Methods("DELETE")
	// t.HandleFunc("/threads/{threadID}/messages", c.AddMessageToThread).Methods("POST")
	// t.HandleFunc("/threads/{threadID}/messages/{messageID}", c.DeleteThreadMessage).Methods("DELETE")

	return r
}

type createThreadPayload struct {
	Subject string `validate:"max=255"`
	Users   []*model.UserInput
}

// CreateThread creates a thread.
func (c *Config) CreateThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	if !u.IsRegistered() {
		bjson.WriteJSON(w, map[string]string{
			"message": "You must verify your account before you can create Convos",
		}, http.StatusBadRequest)

		return
	}

	var payload createThreadPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	users, err := c.UserStore.GetOrCreateUsers(ctx, payload.Users)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	thread, err := model.NewThread(html.UnescapeString(payload.Subject), u, users)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.ThreadStore.Commit(ctx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusCreated)
}

// GetThreads gets the user's threads.
func (c *Config) GetThreads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	p := model.GetPagination(r)

	threads, err := c.ThreadStore.GetThreadsByUser(ctx, u, p)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*model.Thread{"threads": threads}, http.StatusOK)
}

// GetThread gets a thread.
func (c *Config) GetThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if thread.OwnerIs(u) || thread.HasUser(u) {
		bjson.WriteJSON(w, thread, http.StatusOK)
		return
	}

	// Otherwise throw a 404.
	bjson.HandleError(w, errors.E(
		errors.Op("handlers.GetThread"),
		errors.Str("no permission"),
		http.StatusNotFound))
}

// DeleteThread allows the owner to delete the thread.
func (c *Config) DeleteThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !thread.OwnerIs(u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.DeleteThread"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	if err := c.ThreadStore.Delete(ctx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

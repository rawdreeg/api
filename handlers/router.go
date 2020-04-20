package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

type Config struct {
	ModelsClient *models.Client
}

// New mounts all of the application's endpoints.
func New(c *Config) http.Handler {
	router := mux.NewRouter()
	router.Use(middleware.WithErrorReporting)

	router.NotFoundHandler = http.HandlerFunc(notFound)
	router.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

	////
	// Inbound email webhook
	////

	router.HandleFunc("/inbound", c.Inbound).Methods("POST")

	////
	// Async tasks
	////

	router.HandleFunc("/tasks/digest", c.CreateDigest)
	router.HandleFunc("/tasks/emails", c.SendEmailsAsync)

	////
	// JSON endpoints
	////

	jsonSubrouter := router.NewRoute().Subrouter()
	jsonSubrouter.Use(bjson.WithJSONRequests, bjson.WithJSONRequestBody)
	jsonSubrouter.HandleFunc("/users", c.CreateUser).Methods("POST")
	jsonSubrouter.HandleFunc("/users/auth", c.AuthenticateUser).Methods("POST")
	jsonSubrouter.HandleFunc("/users/oauth", c.OAuth).Methods("POST")
	jsonSubrouter.HandleFunc("/users/password", c.UpdatePassword).Methods("POST")
	jsonSubrouter.HandleFunc("/users/verify", c.VerifyEmail).Methods("POST")
	jsonSubrouter.HandleFunc("/users/forgot", c.ForgotPassword).Methods("POST")
	jsonSubrouter.HandleFunc("/users/magic", c.MagicLogin).Methods("POST")

	////
	// Transactional endpoints
	////

	txSubrouter := jsonSubrouter.NewRoute().Subrouter()
	txSubrouter.Use(db.WithTransaction)
	txSubrouter.HandleFunc("/events/rsvps", c.MagicRSVP).Methods("POST")
	// Events
	txEventSubrouter := txSubrouter.NewRoute().Subrouter()
	txEventSubrouter.Use(middleware.WithUser(c.ModelsClient), middleware.WithEvent)
	txEventSubrouter.HandleFunc("/events/{eventID}", c.UpdateEvent).Methods("PATCH")
	txEventSubrouter.HandleFunc("/events/{eventID}/users/{userID}", c.AddUserToEvent).Methods("POST")
	txEventSubrouter.HandleFunc("/events/{eventID}/users/{userID}", c.RemoveUserFromEvent).Methods("DELETE")
	txEventSubrouter.HandleFunc("/events/{eventID}/rsvps", c.AddRSVPToEvent).Methods("POST")
	txEventSubrouter.HandleFunc("/events/{eventID}/rsvps", c.RemoveRSVPFromEvent).Methods("DELETE")
	txEventSubrouter.HandleFunc("/events/{eventID}/magic", c.MagicInvite).Methods("POST")
	txEventSubrouter.HandleFunc("/events/{eventID}/magic", c.RollMagicLink).Methods("DELETE")
	// Threads
	txThreadSubrouter := txSubrouter.NewRoute().Subrouter()
	txThreadSubrouter.Use(middleware.WithUser(c.ModelsClient), middleware.WithThread(c.ModelsClient))
	txThreadSubrouter.HandleFunc("/threads/{threadID}", c.UpdateThread).Methods("PATCH")
	txThreadSubrouter.HandleFunc("/threads/{threadID}/users/{userID}", c.AddUserToThread).Methods("POST")
	txThreadSubrouter.HandleFunc("/threads/{threadID}/users/{userID}", c.RemoveUserFromThread).Methods("DELETE")
	txThreadSubrouter.HandleFunc("/threads/{threadID}/messages", c.AddMessageToThread).Methods("POST")
	txThreadSubrouter.HandleFunc("/threads/{threadID}/messages/{messageID}", c.DeleteThreadMessage).Methods("DELETE")

	////
	// JSON & Auth endpoints
	////

	authSubrouter := jsonSubrouter.NewRoute().Subrouter()
	authSubrouter.Use(middleware.WithUser(c.ModelsClient))
	// Users
	authSubrouter.HandleFunc("/users", c.GetCurrentUser).Methods("GET")
	authSubrouter.HandleFunc("/users", c.UpdateUser).Methods("PATCH")
	authSubrouter.HandleFunc("/users/emails", c.AddEmail).Methods("POST")
	authSubrouter.HandleFunc("/users/emails", c.RemoveEmail).Methods("DELETE")
	authSubrouter.HandleFunc("/users/emails", c.MakeEmailPrimary).Methods("PATCH")
	authSubrouter.HandleFunc("/users/resend", c.SendVerifyEmail).Methods("POST")
	authSubrouter.HandleFunc("/users/search", c.UserSearch).Methods("GET")
	authSubrouter.HandleFunc("/users/avatar", c.PutAvatar).Methods("POST")
	authSubrouter.HandleFunc("/users/{userID}", c.GetUser).Methods("GET")
	// Threads
	authSubrouter.HandleFunc("/threads", c.CreateThread).Methods("POST")
	authSubrouter.HandleFunc("/threads", c.GetThreads).Methods("GET")
	// Events
	authSubrouter.HandleFunc("/events", c.CreateEvent).Methods("POST")
	authSubrouter.HandleFunc("/events", c.GetEvents).Methods("GET")
	// Contacts
	authSubrouter.HandleFunc("/contacts", c.GetContacts).Methods("GET")
	authSubrouter.HandleFunc("/contacts/{userID}", c.AddContact).Methods("POST")
	authSubrouter.HandleFunc("/contacts/{userID}", c.RemoveContact).Methods("DELETE")

	////
	// JSON & Auth & Thread endpoints
	////

	threadSubrouter := authSubrouter.NewRoute().Subrouter()
	threadSubrouter.Use(middleware.WithThread(c.ModelsClient))
	threadSubrouter.HandleFunc("/threads/{threadID}", c.GetThread).Methods("GET")
	threadSubrouter.HandleFunc("/threads/{threadID}", c.DeleteThread).Methods("DELETE")
	threadSubrouter.HandleFunc("/threads/{threadID}/messages", c.GetMessagesByThread).Methods("GET")
	threadSubrouter.HandleFunc("/threads/{threadID}/reads", c.MarkThreadAsRead).Methods("POST")

	////
	// JSON & Auth & Event endpoints
	////

	eventSubrouter := authSubrouter.NewRoute().Subrouter()
	eventSubrouter.Use(middleware.WithEvent)
	eventSubrouter.HandleFunc("/events/{eventID}", c.GetEvent).Methods("GET")
	eventSubrouter.HandleFunc("/events/{eventID}", c.DeleteEvent).Methods("DELETE")
	eventSubrouter.HandleFunc("/events/{eventID}/messages", c.GetMessagesByEvent).Methods("GET")
	eventSubrouter.HandleFunc("/events/{eventID}/messages", c.AddMessageToEvent).Methods("POST")
	eventSubrouter.HandleFunc("/events/{eventID}/messages/{messageID}", c.DeleteEventMessage).Methods("DELETE")
	eventSubrouter.HandleFunc("/events/{eventID}/reads", c.MarkEventAsRead).Methods("POST")
	eventSubrouter.HandleFunc("/events/{eventID}/magic", c.GetMagicLink).Methods("GET")

	return middleware.WithLogging(middleware.WithCORS(router))
}

func notFound(w http.ResponseWriter, r *http.Request) {
	bjson.WriteJSON(w, map[string]string{
		"message": "Not found",
	}, http.StatusNotFound)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	bjson.WriteJSON(w, map[string]string{
		"message": "Method not allowed",
	}, http.StatusMethodNotAllowed)
}

package user

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

type Config struct {
	UserStore model.UserStore
	Mail      *mail.Client
	Magic     magic.Client
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/users", c.CreateUser).Methods(http.MethodPost)
	// r.HandleFunc("/users/auth", c.AuthenticateUser).Methods("POST")
	// r.HandleFunc("/users/oauth", c.OAuth).Methods("POST")
	// r.HandleFunc("/users/password", c.UpdatePassword).Methods("POST")
	// r.HandleFunc("/users/verify", c.VerifyEmail).Methods("POST")
	// r.HandleFunc("/users/forgot", c.ForgotPassword).Methods("POST")
	// r.HandleFunc("/users/magic", c.MagicLogin).Methods("POST")

	sub := r.NewRoute().Subrouter()
	sub.Use(middleware.WithUser(c.UserStore))
	sub.HandleFunc("/users", c.GetCurrentUser).Methods(http.MethodGet)
	// r.HandleFunc("/users", c.UpdateUser).Methods("PATCH")
	// r.HandleFunc("/users/emails", c.AddEmail).Methods("POST")
	// r.HandleFunc("/users/emails", c.RemoveEmail).Methods("DELETE")
	// r.HandleFunc("/users/emails", c.MakeEmailPrimary).Methods("PATCH")
	// r.HandleFunc("/users/resend", c.SendVerifyEmail).Methods("POST")
	// r.HandleFunc("/users/search", c.UserSearch).Methods("GET")
	// r.HandleFunc("/users/avatar", c.PutAvatar).Methods("POST")
	sub.HandleFunc("/users/{userID}", c.GetUser).Methods(http.MethodGet)

	return r
}

type createUserPayload struct {
	Email     string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
	FirstName string `validate:"nonzero"`
	LastName  string
	Password  string `validate:"min=8"`
}

// CreateUser creates a new user with a password.
func (c *Config) CreateUser(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handler.CreateUser")
	ctx := r.Context()

	var payload createUserPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Make sure the user is not already registered
	foundUser, found, err := c.UserStore.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleError(w, err)
		return
	} else if found {
		if !foundUser.IsPasswordSet && !foundUser.IsGoogleLinked && !foundUser.IsFacebookLinked {
			// The email is registered but the user has not setup their account.
			// In order to make sure the requestor is who they say thay are and is not
			// trying to gain access to someone else's identity, we lock the account and
			// require that the email be verified before the user can get access.
			foundUser.FirstName = payload.FirstName
			foundUser.LastName = payload.LastName
			foundUser.IsLocked = true

			if err := c.Mail.SendPasswordResetEmail(
				foundUser,
				foundUser.GetPasswordResetMagicLink(c.Magic)); err != nil {
				bjson.HandleError(w, err)
				return
			}

			if err := c.UserStore.Commit(ctx, foundUser); err != nil {
				bjson.HandleError(w, err)
				return
			}

			bjson.WriteJSON(w, map[string]string{
				"message": "Please verify your email to proceed",
			}, http.StatusOK)
			return
		}

		bjson.HandleError(w, errors.E(op,
			errors.Str("already registered"),
			map[string]string{"message": "This email has already been registered"},
			http.StatusBadRequest))
		return
	}

	// Create the user object
	user, err := model.NewUserWithPassword(
		payload.Email,
		payload.FirstName,
		payload.LastName,
		payload.Password)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Save the user object
	if err := c.UserStore.Commit(ctx, user); err != nil {
		bjson.HandleError(w, err)
		return
	}

	err = c.Mail.SendVerifyEmail(user, user.Email, user.GetVerifyEmailMagicLink(c.Magic, user.Email))
	if err != nil {
		bjson.HandleError(w, err)
		return
	}
	// TODO: user.Welcome(ctx)

	bjson.WriteJSON(w, user, http.StatusCreated)
}

// GetCurrentUser is an endpoint that returns the current user.
func (c *Config) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	bjson.WriteJSON(w, u, http.StatusOK)
}

// GetUser is an endpoint that returns the requested user if found.
func (c *Config) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userID := vars["userID"]

	u, err := c.UserStore.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, errors.E(errors.Op("handlers.GetUser"), err, http.StatusNotFound))
		return
	}

	bjson.WriteJSON(w, model.MapUserToUserPartial(u), http.StatusOK)
}

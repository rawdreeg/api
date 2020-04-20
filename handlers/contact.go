package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

func (c *Config) GetContacts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	contacts, err := models.GetContactsByUser(ctx, &u)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.UserPartial{"contacts": contacts}, http.StatusOK)
}

func (c *Config) AddContact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	if !u.IsRegistered() {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.AddContact"),
			errors.Str("not verified"),
			map[string]string{"message": "You must register before you can add contacts"},
			http.StatusBadRequest))
		return
	}

	userToBeAdded, err := c.ModelsClient.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.AddContact(&userToBeAdded); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.Commit(ctx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, models.MapUserToUserPartial(&userToBeAdded), http.StatusCreated)
}

func (c *Config) RemoveContact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, err := c.ModelsClient.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.RemoveContact(&userToBeRemoved); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.Commit(ctx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, models.MapUserToUserPartial(&userToBeRemoved), http.StatusOK)
}

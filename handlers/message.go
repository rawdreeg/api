package handlers

import (
	"html"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	notif "github.com/hiconvo/api/notifications"
	"github.com/hiconvo/api/utils/bjson"
	og "github.com/hiconvo/api/utils/opengraph"
	"github.com/hiconvo/api/utils/validate"
)

// GetMessagesByThread Endpoint: GET /threads/{id}/messages

// GetMessagesByThread gets the messages from the given thread.
func (c *Config) GetMessagesByThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if !(thread.OwnerIs(&u) || thread.HasUser(&u)) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.GetMessagesByThread"),
			errors.Str("no permission"),
			http.StatusNotFound))
		return
	}

	messages, err := c.ModelsClient.GetMessagesByThread(ctx, &thread)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.Message{"messages": messages}, http.StatusOK)
}

// AddMessageToThread Endpoint: POST /threads/:id/messages
//
// Request payload:
type createMessagePayload struct {
	Body string `validate:"nonzero"`
	Blob string
}

// AddMessageToThread adds a message to the given thread.
func (c *Config) AddMessageToThread(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.AddMessageToThread")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	tx, _ := db.TransactionFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	// Validate raw data
	var payload createMessagePayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	// Check permissions
	if !(thread.OwnerIs(&u) || thread.HasUser(&u)) {
		bjson.HandleError(w, errors.E(op, http.StatusNotFound, errors.Str("NoPermission")))
		return
	}

	var photoKey string
	var err error
	if payload.Blob != "" {
		photoURL, err := c.Storage.PutPhotoFromBlob(ctx, thread.ID, payload.Blob)
		if err != nil {
			bjson.HandleError(w, errors.E(op, err))
			return
		}
		photoKey = c.Storage.GetKeyFromPhotoURL(photoURL)
	}

	messageBody := html.UnescapeString(payload.Body)
	link := og.Extract(ctx, messageBody)

	message, err := c.ModelsClient.NewThreadMessage(
		&u,
		&thread,
		messageBody,
		photoKey,
		link,
	)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if thread.ResponseCount == 1 {
		// Name the thread after the link, if included
		if message.HasLink() && message.Link.Title != "" {
			thread.Subject = message.Link.Title
		}
	}

	if err := message.Commit(ctx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := thread.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	// Send a notification for all later responses
	if err := c.Ntf.Put(notif.Notification{
		UserKeys:   notif.FilterKey(thread.UserKeys, u.Key),
		Actor:      u.FullName,
		Verb:       notif.NewMessage,
		Target:     notif.Thread,
		TargetID:   thread.ID,
		TargetName: thread.Subject,
	}); err != nil {
		// Log the error but don't fail the request
		log.Alarm(err)
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}

// DeleteThreadMessage Endpoint: DELETE /threads/:threadId/messages/:messageId
func (c *Config) DeleteThreadMessage(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.DeleteThreadMessage")
	ctx := r.Context()
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	vars := mux.Vars(r)
	id := vars["messageID"]

	messages, err := c.ModelsClient.GetMessagesByThread(ctx, &thread)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	var m *models.Message
	for i := range messages {
		if messages[i].ID == id {
			m = messages[i]
		}
	}

	if m == nil {
		bjson.HandleError(w, errors.E(op, errors.Str("NotChild"), http.StatusNotFound))
		return
	}

	// Check permissions
	if !(m.OwnerIs(&u)) {
		bjson.HandleError(w, errors.E(op, errors.Str("NoPermission"), http.StatusNotFound))
		return
	}

	// The top message from a thread cannot be deleted.
	if messages[0].Key.Equal(m.Key) {
		bjson.HandleError(w, errors.E(op, errors.Str("DeleteHeadMessage"),
			map[string]string{"message": "You cannot delete this message"},
			http.StatusBadRequest))
		return
	}

	if err := m.Delete(ctx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	thread.ResponseCount--

	if _, err := thread.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, m, http.StatusOK)
}

// GetMessagesByEvent Endpoint: GET /events/{id}/messages

// GetMessagesByEvent gets the messages from the given thread.
func (c *Config) GetMessagesByEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !(event.OwnerIs(&u) || event.HasUser(&u)) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.GetMessagesByEvent"),
			errors.Str("no permission"),
			http.StatusNotFound))
		return
	}

	messages, err := c.ModelsClient.GetMessagesByEvent(ctx, &event)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.Message{"messages": messages}, http.StatusOK)
}

// AddMessageToEvent Endpoint: POST /events/:id/messages

// AddMessageToEvent adds a message to the given thread.
func (c *Config) AddMessageToEvent(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.AddMessageToEvent")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	// Validate raw data
	var payload createMessagePayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Check permissions
	if !(event.OwnerIs(&u) || event.HasUser(&u)) {
		bjson.HandleError(w, errors.E(op,
			errors.Str("no permission"),
			http.StatusNotFound))
		return
	}

	var photoKey string
	var err error
	if payload.Blob != "" {
		photoURL, err := c.Storage.PutPhotoFromBlob(ctx, event.ID, payload.Blob)
		if err != nil {
			bjson.HandleError(w, errors.E(op, err))
			return
		}
		photoKey = c.Storage.GetKeyFromPhotoURL(photoURL)
	}

	message, err := c.ModelsClient.NewEventMessage(
		&u,
		&event,
		html.UnescapeString(payload.Body),
		photoKey)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := message.Commit(ctx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := event.Commit(ctx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.Ntf.Put(notif.Notification{
		UserKeys:   notif.FilterKey(event.UserKeys, u.Key),
		Actor:      u.FullName,
		Verb:       notif.NewMessage,
		Target:     notif.Event,
		TargetID:   event.ID,
		TargetName: event.Name,
	}); err != nil {
		// Log the error but don't fail the request
		log.Alarm(err)
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}

// DeleteEventMessage Endpoint: DELETE /events/:eventId/messages/:messageId
func (c *Config) DeleteEventMessage(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.DeleteEventMessage")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	vars := mux.Vars(r)
	id := vars["messageID"]

	m, err := c.ModelsClient.GetMessageByID(ctx, id)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err, http.StatusNotFound))
		return
	}

	if !event.Key.Equal(m.ParentKey) {
		bjson.HandleError(w, errors.E(op, errors.Str("MessageNotInEvent"), http.StatusNotFound))
		return
	}

	// Check permissions
	if !(m.OwnerIs(&u)) {
		bjson.HandleError(w, errors.E(op, errors.Str("NoPermission"), http.StatusNotFound))
		return
	}

	if err := m.Delete(ctx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	m.User = models.MapUserToUserPartial(&u)

	bjson.WriteJSON(w, m, http.StatusOK)
}

// DeletePhotoFromMessage Endpoint: DELETE /messages/:id/photos
//
// Request payload:
type deleteMessagePayload struct {
	Key string
}

// DeletePhotoFromMessage deletes a photo from the given message
func (c *Config) DeletePhotoFromMessage(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.DeletePhotoFromMessage")
	ctx := r.Context()
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)
	vars := mux.Vars(r)
	id := vars["messageID"]

	// Validate raw data
	var payload deleteMessagePayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	m, err := c.ModelsClient.GetMessageByID(ctx, id)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err, http.StatusNotFound))
		return
	}

	// Check permissions
	if !(m.OwnerIs(&u)) {
		bjson.HandleError(w, errors.E(op, errors.Str("no permission"), http.StatusNotFound))
		return
	}

	key := c.Storage.GetKeyFromPhotoURL(payload.Key)

	if !m.HasPhotoKey(key) {
		bjson.HandleError(w, errors.E(op, errors.Str("no photo in message"), http.StatusBadRequest))
		return
	}

	if err := m.DeletePhoto(ctx, key); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, m, http.StatusOK)
}

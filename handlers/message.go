package handlers

import (
	"html"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	notif "github.com/hiconvo/api/notifications"
	"github.com/hiconvo/api/storage"
	"github.com/hiconvo/api/utils/bjson"
	og "github.com/hiconvo/api/utils/opengraph"
	"github.com/hiconvo/api/utils/validate"
)

var (
	errMsgCreateMessage = map[string]string{"message": "Could not create message"}
	errMsgSaveMessage   = map[string]string{"message": "Could not save message"}
	errMsgSendMessage   = map[string]string{"message": "Could not send message"}
	errMsgGetMessages   = map[string]string{"message": "Could not get messages"}
	errMsgGetMessage    = map[string]string{"message": "Could not find message"}
)

// GetMessagesByThread Endpoint: GET /threads/{id}/messages

// GetMessagesByThread gets the messages from the given thread.
func GetMessagesByThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if !(thread.OwnerIs(&u) || thread.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	// TODO: Paginate
	messages, merr := models.GetMessagesByThread(ctx, &thread)
	if merr != nil {
		bjson.HandleInternalServerError(w, merr, errMsgGetMessages)
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
func AddMessageToThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	// Validate raw data
	var payload createMessagePayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Check permissions
	if !(thread.OwnerIs(&u) || thread.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	var photoKey string
	var err error
	if payload.Blob != "" {
		photoKey, err = storage.PutPhotoFromBlob(ctx, thread.ID, payload.Blob)
		if err != nil {
			bjson.WriteJSON(w, errMsgUpload, http.StatusBadRequest)
			return
		}
	}

	messageBody := html.UnescapeString(payload.Body)
	link := og.Extract(ctx, messageBody)

	message, err := models.NewThreadMessage(
		&u,
		&thread,
		messageBody,
		photoKey,
		link,
	)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreateMessage)
		return
	}

	// Mark the message as read if it is the first message.
	// Since it will be emailed to all members, it would be redundant
	// to send in the daily digest.
	if thread.ResponseCount == 1 {
		for i := range thread.UserKeys {
			models.MarkAsRead(&message, thread.UserKeys[i])
		}

		// Name the thread after the link, if included
		if message.HasLink() && message.Link.Title != "" {
			thread.Subject = message.Link.Title
		}
	}

	if err := message.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if err := thread.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if thread.ResponseCount == 1 {
		// Only send the first message as an email
		if err := thread.SendAsync(ctx); err != nil {
			bjson.HandleInternalServerError(w, err, errMsgSendMessage)
			return
		}
	} else {
		// Send a notification for all later responses
		if err := notif.Put(notif.Notification{
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
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}

// GetMessagesByEvent Endpoint: GET /events/{id}/messages

// GetMessagesByEvent gets the messages from the given thread.
func GetMessagesByEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !(event.OwnerIs(&u) || event.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	// TODO: Paginate
	messages, merr := models.GetMessagesByEvent(ctx, &event)
	if merr != nil {
		bjson.HandleInternalServerError(w, merr, errMsgGetMessages)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.Message{"messages": messages}, http.StatusOK)
}

// AddMessageToEvent Endpoint: POST /events/:id/messages

// AddMessageToEvent adds a message to the given thread.
func AddMessageToEvent(w http.ResponseWriter, r *http.Request) {
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
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	var photoKey string
	var err error
	if payload.Blob != "" {
		photoKey, err = storage.PutPhotoFromBlob(ctx, event.ID, payload.Blob)
		if err != nil {
			bjson.WriteJSON(w, errMsgUpload, http.StatusBadRequest)
			return
		}
	}

	message, err := models.NewEventMessage(
		&u,
		&event,
		html.UnescapeString(payload.Body),
		photoKey)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreateMessage)
		return
	}

	if err := message.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if err := notif.Put(notif.Notification{
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

// DeletePhotoFromMessage Endpoint: DELETE /messages/:id/photos
//
// Request payload:
type deleteMessagePayload struct {
	Key string
}

// DeletePhotoFromMessage deletes a photo from the given message
func DeletePhotoFromMessage(w http.ResponseWriter, r *http.Request) {
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

	m, err := models.GetMessageByID(ctx, id)
	if err != nil {
		bjson.WriteJSON(w, errMsgGetMessage, http.StatusNotFound)
		return
	}

	// Check permissions
	if !(m.OwnerIs(&u)) {
		bjson.WriteJSON(w, errMsgGetMessage, http.StatusNotFound)
		return
	}

	key := storage.GetKeyFromPhotoURL(payload.Key)

	if !m.HasPhotoKey(key) {
		bjson.WriteJSON(w, errMsgGetMessage, http.StatusBadRequest)
		return
	}

	if err := m.DeletePhoto(ctx, key); err != nil {
		bjson.HandleInternalServerError(w, err, map[string]string{
			"message": "Could not delete photo",
		})
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	bjson.WriteJSON(w, m, http.StatusOK)
}

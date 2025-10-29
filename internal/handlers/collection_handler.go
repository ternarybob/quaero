package handlers

import (
	"context"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

type CollectionHandler struct {
	eventService interfaces.EventService
	logger       arbor.ILogger
}

func NewCollectionHandler(
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *CollectionHandler {
	return &CollectionHandler{
		eventService: eventService,
		logger:       logger,
	}
}

// triggerCollection is a helper that publishes a collection event
func (h *CollectionHandler) triggerCollection(w http.ResponseWriter, source, logMsg, successMsg string) {
	h.logger.Info().Msg(logMsg)

	ctx := context.Background()
	event := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: map[string]string{"source": source},
	}

	if err := h.eventService.Publish(ctx, event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to publish collection event")
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteSuccess(w, successMsg)
}

func (h *CollectionHandler) SyncJiraHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.triggerCollection(
		w,
		"manual_jira",
		"Manual Jira sync triggered via event",
		"Jira collection event published (async)",
	)
}

func (h *CollectionHandler) SyncConfluenceHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.triggerCollection(
		w,
		"manual_confluence",
		"Manual Confluence sync triggered via event",
		"Confluence collection event published (async)",
	)
}

func (h *CollectionHandler) SyncAllHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.triggerCollection(
		w,
		"manual_all",
		"Manual full sync triggered (Jira + Confluence) via event",
		"Collection event published (async) - Jira and Confluence will run in parallel",
	)
}

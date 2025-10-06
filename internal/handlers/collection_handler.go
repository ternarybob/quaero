package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/collection"
)

type CollectionHandler struct {
	coordinator  *collection.CoordinatorService
	eventService interfaces.EventService
	logger       arbor.ILogger
}

func NewCollectionHandler(
	coordinator *collection.CoordinatorService,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *CollectionHandler {
	return &CollectionHandler{
		coordinator:  coordinator,
		eventService: eventService,
		logger:       logger,
	}
}

func (h *CollectionHandler) SyncJiraHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Manual Jira sync triggered via event")

	ctx := context.Background()
	event := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: map[string]string{"source": "manual_jira"},
	}

	if err := h.eventService.Publish(ctx, event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to publish collection event")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Jira collection event published (async)",
	})
}

func (h *CollectionHandler) SyncConfluenceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Manual Confluence sync triggered via event")

	ctx := context.Background()
	event := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: map[string]string{"source": "manual_confluence"},
	}

	if err := h.eventService.Publish(ctx, event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to publish collection event")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Confluence collection event published (async)",
	})
}

func (h *CollectionHandler) SyncAllHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Manual full sync triggered (Jira + Confluence) via event")

	ctx := context.Background()
	event := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: map[string]string{"source": "manual_all"},
	}

	if err := h.eventService.Publish(ctx, event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to publish collection event")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Collection event published (async) - Jira and Confluence will run in parallel",
	})
}

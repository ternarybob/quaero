package queue

import (
	"github.com/ternarybob/quaero/internal/models"
)

// ErrNoMessage is returned when the queue is empty
var ErrNoMessage = models.ErrNoMessage

// Message is an alias for models.QueueMessage to maintain backward compatibility
// within the queue package. New code should use models.QueueMessage directly.
type Message = models.QueueMessage

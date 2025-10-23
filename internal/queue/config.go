package queue

import "time"

// Config holds configuration for the queue manager
type Config struct {
	// PollInterval is how often workers poll for messages
	PollInterval time.Duration

	// Concurrency is the number of concurrent workers
	Concurrency int

	// VisibilityTimeout is the message visibility timeout for redelivery
	VisibilityTimeout time.Duration

	// MaxReceive is the maximum times a message can be received before dead-letter
	MaxReceive int

	// QueueName is the name of the queue in goqite table
	QueueName string
}

// NewDefaultConfig creates a queue configuration with sensible defaults
func NewDefaultConfig() Config {
	return Config{
		PollInterval:      1 * time.Second,
		Concurrency:       5,
		VisibilityTimeout: 5 * time.Minute,
		MaxReceive:        3,
		QueueName:         "quaero_jobs",
	}
}

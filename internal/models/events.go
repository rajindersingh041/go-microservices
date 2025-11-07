package models

import "time"

// Event is now a structured log
type Event struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Source    string            `json:"source"`
	Message   string            `json:"message"`
	Context   map[string]string `json:"context"` // <-- THE NEW FIELD
}
package entity

import "time"

type Envelope struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      EntryType         `json:"type"`
	Meta      map[string]string `json:"meta,omitempty"`
	Payload   []byte            `json:"payload"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

package domain

import "time"

type Message struct {
	ID        string
	Text      string
	Status    string
	CreatedAt time.Time
}

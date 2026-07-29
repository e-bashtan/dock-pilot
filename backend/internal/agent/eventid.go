package agent

import "github.com/google/uuid"

func newEventID() uuid.UUID {
	return uuid.New()
}

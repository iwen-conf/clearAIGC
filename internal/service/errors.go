package service

import "errors"

var (
	ErrInvalidRequest     = errors.New("invalid request")
	ErrSessionNotFound    = errors.New("session not found")
	ErrRoundNotFound      = errors.New("round not found")
	ErrSessionLocked      = errors.New("session locked")
	ErrAllRoundsCompleted = errors.New("all rounds completed")
	ErrNoActiveRound      = errors.New("no active round")
	ErrNoPausedRound      = errors.New("no paused round")
	ErrUnsupportedFormat  = errors.New("unsupported format")
	ErrAgentNotFound      = errors.New("agent not found")

	errCardNotFound = errors.New("card not found")
)

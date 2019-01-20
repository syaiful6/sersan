package sersan

import (
	"fmt"
)

type SessionAlreadyExists struct {
	OldSession, NewSession *Session
}

func (err *SessionAlreadyExists) Error() string {
	return fmt.Sprintf("There is already exists a session with the same session ID: %s", err.NewSession.ID) 
}

type SessionDoesNotExist struct {
	Session *Session
}

func (err *SessionDoesNotExist) Error() string {
	return fmt.Sprintf("There is no session with the given session ID: %s", err.Session.ID)
}
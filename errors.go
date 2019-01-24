package sersan

import (
	"fmt"
)

type SessionAlreadyExists struct {
	ID string
}

func (err SessionAlreadyExists) Error() string {
	return fmt.Sprintf("There is already exists a session with the same session ID: %s", err.ID)
}

type SessionDoesNotExist struct {
	ID string
}

func (err SessionDoesNotExist) Error() string {
	return fmt.Sprintf("There is no session with the given session ID: %s", err.ID)
}

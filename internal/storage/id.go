package storage

import (
	"crypto/rand"
	"fmt"
)

// newID generates a random 16-character hex identifier.
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

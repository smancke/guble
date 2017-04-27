package go_uuid

import (
	"errors"
	"fmt"
	"crypto/rand"
)

var (
	ErrRandomnessSourceRead = errors.New("randomness source cannot be read")
)

func New() (string, error) {
	b := make([]byte, 16)
	n, err := rand.Read(b)
	if err != nil || n <= 0 {
		return "", ErrRandomnessSourceRead
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

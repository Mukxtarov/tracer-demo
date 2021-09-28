package remote

import (
	"fmt"
	"github.com/pkg/errors"
)

var (
	ErrConnectionFailed       = errors.New("could not connect to service")
	ErrUnexpectedResponseData = errors.New("service returned unexpected data")
)

type Error struct {
	Err  error
	Info string
}

func (e Error) Error() string {
	return fmt.Sprintf("%v: %v", e.Err.Error(), e.Info)
}

func (e Error) Unwrap() error { return e.Err }

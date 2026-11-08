package domain

import (
	"errors"
	"fmt"
)

var (
	ErrTargetNotFound  = errors.New("target not found")
	ErrCheckInProgress = errors.New("check already in progress for this target")
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}

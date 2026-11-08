package service

import (
	"fmt"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const (
	defaultListLimit = 100
	maxListLimit     = 1000
)

func resolveLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultListLimit, nil
	}
	if *limit < 1 || *limit > maxListLimit {
		return 0, &domain.ValidationError{Fields: map[string]string{
			"limit": fmt.Sprintf("must be between 1 and %d", maxListLimit),
		}}
	}
	return *limit, nil
}

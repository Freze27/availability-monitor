package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const (
	maxNameLength      = 200
	maxIntervalSeconds = 7 * 24 * 60 * 60
)

type TargetRepository interface {
	CreateTarget(ctx context.Context, t domain.NewTarget) (domain.Target, error)
	GetTarget(ctx context.Context, id int64) (domain.Target, error)
	ListTargets(ctx context.Context) ([]domain.Target, error)
	UpdateTarget(ctx context.Context, id int64, u domain.TargetUpdate) (domain.Target, error)
	DeleteTarget(ctx context.Context, id int64) error
}

type TargetService struct {
	repo TargetRepository
}

func NewTargetService(repo TargetRepository) *TargetService {
	return &TargetService{repo: repo}
}

func (s *TargetService) Create(ctx context.Context, t domain.NewTarget) (domain.Target, error) {
	fields := map[string]string{}

	name, msg := normalizeName(t.Name)
	if msg != "" {
		fields["name"] = msg
	}
	rawURL, msg := normalizeURL(t.URL)
	if msg != "" {
		fields["url"] = msg
	}
	if msg := validateInterval(t.IntervalSeconds); msg != "" {
		fields["interval_seconds"] = msg
	}
	if len(fields) > 0 {
		return domain.Target{}, &domain.ValidationError{Fields: fields}
	}

	return s.repo.CreateTarget(ctx, domain.NewTarget{
		Name:            name,
		URL:             rawURL,
		IntervalSeconds: t.IntervalSeconds,
	})
}

func (s *TargetService) Get(ctx context.Context, id int64) (domain.Target, error) {
	return s.repo.GetTarget(ctx, id)
}

func (s *TargetService) List(ctx context.Context) ([]domain.Target, error) {
	return s.repo.ListTargets(ctx)
}

func (s *TargetService) Update(ctx context.Context, id int64, u domain.TargetUpdate) (domain.Target, error) {
	if u.Name == nil && u.URL == nil && u.IntervalSeconds == nil {
		return domain.Target{}, &domain.ValidationError{Fields: map[string]string{
			"body": "at least one of name, url, interval_seconds must be provided",
		}}
	}

	fields := map[string]string{}
	var normalized domain.TargetUpdate

	if u.Name != nil {
		name, msg := normalizeName(*u.Name)
		if msg != "" {
			fields["name"] = msg
		}
		normalized.Name = &name
	}
	if u.URL != nil {
		rawURL, msg := normalizeURL(*u.URL)
		if msg != "" {
			fields["url"] = msg
		}
		normalized.URL = &rawURL
	}
	if u.IntervalSeconds != nil {
		if msg := validateInterval(*u.IntervalSeconds); msg != "" {
			fields["interval_seconds"] = msg
		}
		normalized.IntervalSeconds = u.IntervalSeconds
	}
	if len(fields) > 0 {
		return domain.Target{}, &domain.ValidationError{Fields: fields}
	}

	return s.repo.UpdateTarget(ctx, id, normalized)
}

func (s *TargetService) Delete(ctx context.Context, id int64) error {
	return s.repo.DeleteTarget(ctx, id)
}

func normalizeName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "must not be empty"
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return "", fmt.Sprintf("must not exceed %d characters", maxNameLength)
	}
	return name, ""
}

func normalizeURL(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "must not be empty"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "is not a valid URL"
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "must start with http:// or https://"
	}
	if u.Host == "" {
		return "", "must contain a host"
	}
	return raw, ""
}

func validateInterval(seconds int) string {
	if seconds <= 0 || seconds > maxIntervalSeconds {
		return fmt.Sprintf("must be between 1 and %d", maxIntervalSeconds)
	}
	return ""
}

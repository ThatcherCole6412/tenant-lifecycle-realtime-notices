package notify

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type LifecycleEvent struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	AccountID string `json:"account_id"`
	Kind      string `json:"kind"`
	Actor     string `json:"actor"`
}

type Notification struct {
	Channel   string `json:"channel"`
	Event     string `json:"event"`
	AccountID string `json:"account_id"`
	Title     string `json:"title"`
	Severity  string `json:"severity"`
}

type Publisher interface {
	Publish(context.Context, string, string, string, string, any) error
}

type LifecycleService struct{ publisher Publisher }

func NewLifecycleService(p Publisher) *LifecycleService { return &LifecycleService{publisher: p} }

func TenantChannel(tenantID string) (string, error) {
	if tenantID == "" || !safeID.MatchString(tenantID) {
		return "", errors.New("tenant_id must be URL-safe")
	}
	return "tenant-" + tenantID, nil
}

func Decide(input LifecycleEvent) (Notification, error) {
	if input.ID == "" || input.AccountID == "" {
		return Notification{}, errors.New("id and account_id are required")
	}
	channel, err := TenantChannel(input.TenantID)
	if err != nil {
		return Notification{}, err
	}
	n := Notification{Channel: channel, AccountID: input.AccountID}
	switch input.Kind {
	case "onboarding.completed":
		n.Event, n.Title, n.Severity = "account.ready", "Account onboarding completed", "info"
	case "account.review_required":
		n.Event, n.Title, n.Severity = "account.review_required", "Account review required", "warning"
	case "admin.account_suspended":
		if input.Actor == "" {
			return Notification{}, errors.New("actor is required for admin operations")
		}
		n.Event, n.Title, n.Severity = "account.suspended", "Account access suspended", "critical"
	default:
		return Notification{}, fmt.Errorf("unsupported lifecycle event %q", input.Kind)
	}
	return n, nil
}

func (s *LifecycleService) Notify(ctx context.Context, input LifecycleEvent) (Notification, error) {
	n, err := Decide(input)
	if err != nil {
		return Notification{}, err
	}
	data := map[string]string{"title": n.Title, "severity": n.Severity, "source_event_id": input.ID}
	if err := s.publisher.Publish(ctx, input.ID, n.Channel, n.Event, n.AccountID, data); err != nil {
		return Notification{}, err
	}
	return n, nil
}

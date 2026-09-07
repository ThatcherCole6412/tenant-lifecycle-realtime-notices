package notify

import "testing"

func TestDecideLifecycleNotification(t *testing.T) {
	tests := []struct {
		name      string
		input     LifecycleEvent
		wantEvent string
		wantLevel string
		wantErr   bool
	}{
		{"onboarding", LifecycleEvent{ID: "evt-1", TenantID: "acme", AccountID: "acct-7", Kind: "onboarding.completed"}, "account.ready", "info", false},
		{"compliance review", LifecycleEvent{ID: "evt-2", TenantID: "acme", AccountID: "acct-7", Kind: "account.review_required"}, "account.review_required", "warning", false},
		{"admin suspension", LifecycleEvent{ID: "evt-3", TenantID: "acme", AccountID: "acct-7", Kind: "admin.account_suspended", Actor: "admin-4"}, "account.suspended", "critical", false},
		{"admin audit identity required", LifecycleEvent{ID: "evt-4", TenantID: "acme", AccountID: "acct-7", Kind: "admin.account_suspended"}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decide(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decide() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Event != tt.wantEvent || got.Severity != tt.wantLevel {
				t.Fatalf("Decide() = event %q severity %q", got.Event, got.Severity)
			}
			if !tt.wantErr && got.Channel != "tenant-acme" {
				t.Fatalf("channel = %q", got.Channel)
			}
		})
	}
}

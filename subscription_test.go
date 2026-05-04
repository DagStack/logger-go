package logger_test

import (
	"testing"

	"go.dagstack.dev/logger"
)

func TestSubscriptionInactiveFalseDefault(t *testing.T) {
	sub := logger.NewInactiveSubscription("x", "noop")
	if sub.Active {
		t.Fatalf("Active = true, want false")
	}
	if sub.InactiveReason != "noop" {
		t.Fatalf("InactiveReason = %q", sub.InactiveReason)
	}
	if sub.Path != "x" {
		t.Fatalf("Path = %q", sub.Path)
	}
}

func TestSubscriptionUnsubscribeCallsImpl(t *testing.T) {
	calls := 0
	sub := logger.NewSubscription("x", func() { calls++ })
	if !sub.Active {
		t.Fatalf("active subscription not active")
	}
	sub.Unsubscribe()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSubscriptionUnsubscribeIdempotentMultiple(t *testing.T) {
	calls := 0
	sub := logger.NewSubscription("x", func() { calls++ })
	sub.Unsubscribe()
	sub.Unsubscribe()
	sub.Unsubscribe()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSubscriptionInactiveUnsubscribeNoop(t *testing.T) {
	sub := logger.NewInactiveSubscription("x", "noop")
	sub.Unsubscribe() // does not panic
}

func TestSubscriptionNilReceiverNoop(t *testing.T) {
	var sub *logger.Subscription
	sub.Unsubscribe()
}

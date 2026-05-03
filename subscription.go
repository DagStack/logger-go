package logger

// Subscription is a placeholder handle returned by Logger.OnReconfigure per
// spec ADR-0001 §7.2.
//
// In Phase 1 watch-based reconfigure is not implemented — every subscription
// is constructed with Active=false and InactiveReason populated. The handle
// is forward-compatible: when Phase 2 introduces a Watcher (file or admin
// API), the same Subscription type carries the active subscription with a
// real Unsubscribe callback.
type Subscription struct {
	// Path echoes the subscription path for introspection.
	Path string

	// Active is true iff a watch-capable backend is registered AND covers
	// the subscription path. Always false in Phase 1.
	Active bool

	// InactiveReason carries a human-readable diagnostic when Active=false.
	// Phase 1 sets it to a fixed message:
	//
	//   "Phase 1 logger does not support watch-based reconfigure"
	InactiveReason string

	// unsubscribe is the cancellation callback; idempotent.
	unsubscribe  func()
	unsubscribed bool
}

// NewInactiveSubscription constructs a Subscription whose Active field is
// false. Use this to signal that a subscription was accepted but will never
// fire — typically because watch is not implemented in this binding phase.
func NewInactiveSubscription(path, reason string) *Subscription {
	return &Subscription{Path: path, Active: false, InactiveReason: reason}
}

// NewSubscription constructs an active Subscription bound to the given
// cancellation callback. Reserved for Phase 2+ Watcher implementations.
func NewSubscription(path string, unsubscribe func()) *Subscription {
	return &Subscription{Path: path, Active: true, unsubscribe: unsubscribe}
}

// Unsubscribe cancels the subscription. Idempotent — subsequent calls are
// no-op. After Unsubscribe returns, the callback is guaranteed not to fire.
func (s *Subscription) Unsubscribe() {
	if s == nil || s.unsubscribed {
		return
	}
	s.unsubscribed = true
	if s.unsubscribe != nil {
		s.unsubscribe()
	}
}

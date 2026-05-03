package logger

// ResetRegistryForTests clears the global Logger registry — exposed only to
// internal-package tests for isolation between cases. Not part of the
// public API.
func ResetRegistryForTests() {
	resetRegistry()
}

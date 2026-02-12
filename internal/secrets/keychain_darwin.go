//go:build darwin

package secrets

// IsKeychainLockedError returns false for the file-backed secret store.
func IsKeychainLockedError(_ string) bool { return false }

// CheckKeychainLocked returns false for the file-backed secret store.
func CheckKeychainLocked() bool { return false }

// UnlockKeychain is a no-op for the file-backed secret store.
func UnlockKeychain() error { return nil }

// EnsureKeychainAccess is a no-op for the file-backed secret store.
func EnsureKeychainAccess() error { return nil }

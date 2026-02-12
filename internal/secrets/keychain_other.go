//go:build !darwin

package secrets

func IsKeychainLockedError(_ string) bool { return false }
func CheckKeychainLocked() bool           { return false }
func UnlockKeychain() error               { return nil }
func EnsureKeychainAccess() error         { return nil }

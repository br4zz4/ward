package sops

import (
	"fmt"
)

// RequireKeyDecryptor is the fallback used when no encryption key is available.
// It reads .plain.ward files as plaintext (delegating to PlainDecryptor) but
// errors on any other .ward file, so encrypted content is never silently parsed
// as YAML. The error is sentinel-detectable via IsNoKeyError.
type RequireKeyDecryptor struct{}

func (RequireKeyDecryptor) Decrypt(path string) ([]byte, error) {
	if isPlainWardName(path) {
		return PlainDecryptor{}.Decrypt(path)
	}
	return nil, &NoKeyError{Path: path}
}

// MissingKeyDecryptor is used for a vault that declares a key source (key_env or
// key_file) which is unavailable at runtime. Reads behave like
// RequireKeyDecryptor — encrypted files surface a NoKeyError so callers can skip
// the vault with a warning instead of aborting — but it carries a Hint telling
// the user how to supply the key, and it is deliberately not an Encryptor so
// writes cannot silently overwrite encrypted content with plaintext.
type MissingKeyDecryptor struct{ Hint string }

func (d MissingKeyDecryptor) Decrypt(path string) ([]byte, error) {
	if isPlainWardName(path) {
		return PlainDecryptor{}.Decrypt(path)
	}
	return nil, &NoKeyError{Path: path, Hint: d.Hint}
}

// NoKeyError signals that path is encrypted but no key was available.
// Hint, when set, explains how to provide the key.
type NoKeyError struct {
	Path string
	Hint string
}

func (e *NoKeyError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: no encryption key configured for this file — %s", e.Path, e.Hint)
	}
	return fmt.Sprintf("%s: no encryption key configured for this file", e.Path)
}

// NoKeyHint returns the hint carried by a NoKeyError in err's chain, or "".
func NoKeyHint(err error) string {
	for err != nil {
		if nke, ok := err.(*NoKeyError); ok {
			return nke.Hint
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return ""
		}
		err = u.Unwrap()
	}
	return ""
}

// IsNoKeyError reports whether err is (or wraps) a NoKeyError.
func IsNoKeyError(err error) bool {
	for err != nil {
		if _, ok := err.(*NoKeyError); ok {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

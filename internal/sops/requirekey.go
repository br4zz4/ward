package sops

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RequireKeyDecryptor is the fallback used when no encryption key is available.
// It reads .plain.ward files as plaintext (delegating to PlainDecryptor) but
// errors on any other .ward file, so encrypted content is never silently parsed
// as YAML. The error is sentinel-detectable via IsNoKeyError.
type RequireKeyDecryptor struct{}

func (RequireKeyDecryptor) Decrypt(path string) ([]byte, error) {
	if strings.HasSuffix(filepath.Base(path), ".plain.ward") {
		return PlainDecryptor{}.Decrypt(path)
	}
	return nil, &NoKeyError{Path: path}
}

// NoKeyError signals that path is encrypted but no key was available.
type NoKeyError struct{ Path string }

func (e *NoKeyError) Error() string {
	return fmt.Sprintf("%s: no encryption key configured for this file", e.Path)
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

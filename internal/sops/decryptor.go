package sops

// Decryptor decrypts a .ward file and returns its raw YAML bytes.
type Decryptor interface {
	Decrypt(path string) ([]byte, error)
}

package secrets

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/br4zz4/ward/internal/sops"
)

// LineMap maps dot-path → line number in the source file.
type LineMap map[string]int

// ParsedFile holds the decoded content of a .ward file before merging.
type ParsedFile struct {
	File     string
	Data     map[string]interface{}
	Lines    LineMap  // dot-path → line number
	RawLines []string // source lines for snippet display
}

// Load decrypts and parses a .ward file into a ParsedFile.
// vaultName and vaultRoot identify the vault; used to derive the key prefix
// for file-secrets so they merge at the correct dot-path.
func Load(path, vaultName, vaultRoot string, dec sops.Decryptor) (ParsedFile, error) {
	raw, err := dec.Decrypt(path)
	if err != nil {
		return ParsedFile{}, fmt.Errorf("decrypting %s: %w", path, err)
	}

	// File-secrets store raw content, not YAML structure.
	// Their dot-path is: vaultName + subdir segments + key derived from filename.
	if orig, ok := OriginalFilename(path); ok && !IsPlainFile(path) {
		key := FileKey(orig)
		prefix := append([]string{vaultName}, fileSecretSubdir(path, vaultRoot)...)
		data := nestedMap(prefix, key, strings.TrimRight(string(raw), "\n"))
		dotPath := strings.Join(append(prefix, key), ".")
		return ParsedFile{
			File:     path,
			Data:     data,
			Lines:    LineMap{dotPath: 1},
			RawLines: strings.Split(string(raw), "\n"),
		}, nil
	}

	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return ParsedFile{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	data := map[string]interface{}{}
	lines := LineMap{}
	if len(node.Content) > 0 {
		extractNode(node.Content[0], "", data, lines)
	}

	rawLines := strings.Split(string(raw), "\n")

	return ParsedFile{File: path, Data: data, Lines: lines, RawLines: rawLines}, nil
}

// fileSecretSubdir returns the subdirectory segments between the vault root and
// the file-secret's directory. e.g. vault root ".ward/vaults/app",
// file ".ward/vaults/app/credentials/sa.json.ward" → ["credentials"].
func fileSecretSubdir(filePath, vaultRoot string) []string {
	if vaultRoot == "" {
		return nil
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(vaultRoot, filepath.Dir(absFile))
	if err != nil || rel == "." {
		return nil
	}
	return strings.Split(rel, string(filepath.Separator))
}

// nestedMap wraps value under prefix segments + key as nested maps.
// e.g. prefix=["app","credentials"], key="sa_json", value="..." →
// map["app"]map["credentials"]map["sa_json"]"..."
func nestedMap(prefix []string, key, value string) map[string]interface{} {
	leaf := map[string]interface{}{key: value}
	for i := len(prefix) - 1; i >= 0; i-- {
		leaf = map[string]interface{}{prefix[i]: leaf}
	}
	return leaf
}

// extractNode recursively walks a yaml.Node, populating data and lines.
func extractNode(node *yaml.Node, prefix string, data map[string]interface{}, lines LineMap) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		key := keyNode.Value
		dotPath := key
		if prefix != "" {
			dotPath = prefix + "." + key
		}

		switch valNode.Kind {
		case yaml.MappingNode:
			nested := map[string]interface{}{}
			extractNode(valNode, dotPath, nested, lines)
			data[key] = nested
		case yaml.ScalarNode:
			data[key] = valNode.Value
			lines[dotPath] = valNode.Line
		case yaml.SequenceNode:
			data[key] = valNode.Value
			lines[dotPath] = valNode.Line
		}
	}
}

// LoadAll loads all files using per-file decryptors.
// vaultFor maps each file path to its (vaultName, vaultRoot) for file-secret prefix derivation.
// decFor maps each file path to its Decryptor (allows per-vault keys).
func LoadAll(paths []string, vaultFor func(path string) (string, string), decFor func(path string) sops.Decryptor) ([]ParsedFile, error) {
	files := make([]ParsedFile, 0, len(paths))
	for _, p := range paths {
		name, root := "", ""
		if vaultFor != nil {
			name, root = vaultFor(p)
		}
		dec := decFor(p)
		pf, err := Load(p, name, root, dec)
		if err != nil {
			return nil, err
		}
		files = append(files, pf)
	}
	return files, nil
}

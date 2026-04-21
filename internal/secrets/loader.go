package secrets

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/oporpino/ward/internal/sops"
)

// LineMap maps dot-path → line number in the source file.
type LineMap map[string]int

// ParsedFile holds the decoded content of a .ward file before merging.
type ParsedFile struct {
	File    string
	Data    map[string]interface{}
	Lines   LineMap // dot-path → line number
	RawLines []string // source lines for snippet display
}

// Load decrypts and parses a .ward file into a ParsedFile.
func Load(path string, dec sops.Decryptor) (ParsedFile, error) {
	raw, err := dec.Decrypt(path)
	if err != nil {
		return ParsedFile{}, fmt.Errorf("decrypting %s: %w", path, err)
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

// LoadAll loads all files using the given decryptor.
func LoadAll(paths []string, dec sops.Decryptor) ([]ParsedFile, error) {
	files := make([]ParsedFile, 0, len(paths))
	for _, p := range paths {
		pf, err := Load(p, dec)
		if err != nil {
			return nil, err
		}
		files = append(files, pf)
	}
	return files, nil
}

// Package ward provides the application engine layer between the CLI and the
// secrets package. All orchestration logic lives here; the CLI layer only handles
// flag parsing and output formatting.
package ward

import (
	"fmt"
	"os"

	"github.com/oporpino/ward/internal/config"
	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
)

// Engine orchestrates secret discovery, loading, merging and env-var resolution.
// The zero value is not usable; construct via NewEngine.
type Engine struct {
	cfg *config.Config
	dec sops.Decryptor
}

// NewEngine returns an Engine backed by cfg and dec.
func NewEngine(cfg *config.Config, dec sops.Decryptor) *Engine {
	return &Engine{cfg: cfg, dec: dec}
}

// MergeResult is the outcome of a load-and-merge operation.
type MergeResult struct {
	Tree        map[string]*secrets.Node
	ConflictErr *secrets.ConflictError // non-nil only when called via MergeForView
}

// Merge loads all .ward files from all vaults and merges them using the
// on_conflict mode from the configuration.
func (e *Engine) Merge() (*MergeResult, error) {
	return e.MergeWithConflict("", "")
}

// MergeWithConflict is like Merge but allows overriding the on_conflict behaviour
// and scoping conflict detection to a dot-path prefix.
// When scopePrefix is non-empty, conflicts outside that prefix are silently overridden.
func (e *Engine) MergeWithConflict(onConflict config.OnConflict, scopePrefix string) (*MergeResult, error) {
	files, err := e.load()
	if err != nil {
		return nil, err
	}
	ordered := secrets.SortBySpecificity(files)
	mode := e.conflictMode(onConflict)
	tree, err := secrets.Merge(ordered, mode, scopePrefix)
	if err != nil {
		return nil, err
	}
	return &MergeResult{Tree: tree}, nil
}

// MergeForView is like Merge but always produces a complete tree even when
// conflicts exist. Conflict information is attached to the result so the
// presentation layer can highlight conflicting keys.
func (e *Engine) MergeForView() (*MergeResult, error) {
	files, err := e.load()
	if err != nil {
		return nil, err
	}
	ordered := secrets.SortBySpecificity(files)

	// First pass: detect conflicts without blocking.
	var conflictErr *secrets.ConflictError
	if _, cerr := secrets.Merge(ordered, config.MergeModeError, ""); cerr != nil {
		if ce, ok := cerr.(*secrets.ConflictError); ok {
			conflictErr = ce
		} else {
			return nil, cerr
		}
	}

	// Second pass: override mode so we always get a full tree.
	tree, err := secrets.Merge(ordered, config.MergeModeOverride, "")
	if err != nil {
		return nil, err
	}
	return &MergeResult{Tree: tree, ConflictErr: conflictErr}, nil
}

// Inspect runs a conflict-only merge and returns a ConflictError if any conflicts
// exist, or nil when the set of files is clean.
func (e *Engine) Inspect() error {
	files, err := e.load()
	if err != nil {
		return err
	}
	ordered := secrets.SortBySpecificity(files)
	_, mergeErr := secrets.Merge(ordered, config.MergeModeError, "")
	return mergeErr
}

// EnvVars resolves env vars from the merged result.
// Flat leaf names (DATABASE_URL), or full path if --prefixed.
func (e *Engine) EnvVars(r *MergeResult, prefixed bool) (map[string]secrets.EnvEntry, error) {
	if prefixed {
		return secrets.ToEnvEntries(r.Tree), nil
	}
	return secrets.ToFlatEnvEntries(r.Tree), nil
}

// EnvVarsMap is like EnvVars but returns plain string values (for injection into
// a child process environment).
func (e *Engine) EnvVarsMap(r *MergeResult, prefixed bool) (map[string]string, error) {
	entries, err := e.EnvVars(r, prefixed)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(entries))
	for k, entry := range entries {
		out[k] = entry.Value
	}
	return out, nil
}

// GetAtPath navigates the merged tree by dot-path and returns the node at that
// location, or an error if the path does not exist.
func (e *Engine) GetAtPath(r *MergeResult, dotPath string) (*secrets.Node, error) {
	parts := splitPath(dotPath)
	current := &secrets.Node{Children: r.Tree}
	for _, part := range parts {
		if current.Children == nil {
			return nil, fmt.Errorf("key not found: %s", dotPath)
		}
		next, ok := current.Children[part]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", dotPath)
		}
		current = next
	}
	return current, nil
}

// SourcePaths returns the configured source directory paths.
func (e *Engine) SourcePaths() []string {
	return sourcePaths(e.cfg)
}

// Decrypt returns the plain-text YAML bytes of a .ward file using the
// configured decryptor. For plain (unencrypted) files this is a passthrough.
func (e *Engine) Decrypt(path string) ([]byte, error) {
	return e.dec.Decrypt(path)
}

// Encrypt writes content back to path using the configured encryptor.
// For SopsDecryptor this calls "sops encrypt"; for MockDecryptor it writes plain.
type Encryptor interface {
	Encrypt(path string, plaintext []byte) error
}

// Encrypt re-encrypts plaintext and writes it to path.
// Falls back to a plain write when no real encryptor is configured.
func (e *Engine) Encrypt(path string, plaintext []byte) error {
	if enc, ok := e.dec.(Encryptor); ok {
		return enc.Encrypt(path, plaintext)
	}
	return os.WriteFile(path, plaintext, 0644)
}

// --- internal helpers --------------------------------------------------------

func (e *Engine) load() ([]secrets.ParsedFile, error) {
	paths, err := secrets.Discover(sourcePaths(e.cfg))
	if err != nil {
		return nil, fmt.Errorf("discovering files: %w", err)
	}
	files, err := secrets.LoadAll(paths, e.dec)
	if err != nil {
		return nil, fmt.Errorf("loading files: %w", err)
	}
	return files, nil
}

// conflictMode maps the CLI on_conflict flag to a MergeMode.
// The flag takes precedence over the config value.
func (e *Engine) conflictMode(onConflict config.OnConflict) config.MergeMode {
	effective := e.cfg.OnConflict
	if onConflict != "" {
		effective = onConflict
	}
	if effective == config.OnConflictOverride {
		return config.MergeModeOverride
	}
	return config.MergeModeError
}

func sourcePaths(cfg *config.Config) []string {
	paths := make([]string, len(cfg.Vaults))
	for i, s := range cfg.Vaults {
		paths[i] = s.Path
	}
	return paths
}

func splitPath(dotPath string) []string {
	// strings.Split is in strings package — inline to avoid import for a single call
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(dotPath); i++ {
		if dotPath[i] == '.' {
			parts = append(parts, dotPath[start:i])
			start = i + 1
		}
	}
	return append(parts, dotPath[start:])
}

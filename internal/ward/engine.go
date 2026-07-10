// Package ward provides the application engine layer between the CLI and the
// secrets package. All orchestration logic lives here; the CLI layer only handles
// flag parsing and output formatting.
package ward

import (
	"fmt"
	"os"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/sops"
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
// Merge loads all vaults and merges them. Any conflict is an error.
func (e *Engine) Merge() (*MergeResult, error) {
	return e.MergeScoped("")
}

// MergeScoped is like Merge but scopes conflict detection to a dot-path prefix.
// Conflicts outside that prefix are silently resolved so the scoped path can
// be used without being blocked by unrelated conflicts.
func (e *Engine) MergeScoped(scopePrefix string) (*MergeResult, error) {
	files, err := e.load()
	if err != nil {
		return nil, err
	}
	tree, err := secrets.Merge(files, config.MergeModeError, scopePrefix)
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

	// First pass: detect conflicts without blocking.
	var conflictErr *secrets.ConflictError
	if _, cerr := secrets.Merge(files, config.MergeModeError, ""); cerr != nil {
		if ce, ok := cerr.(*secrets.ConflictError); ok {
			conflictErr = ce
		} else {
			return nil, cerr
		}
	}

	// Second pass: override mode so we always get a full tree.
	tree, err := secrets.Merge(files, config.MergeModeOverride, "")
	if err != nil {
		return nil, err
	}
	// Mark ancestor leafs that are shadowed by deeper leafs with the same key name.
	secrets.MarkShadowed(tree)
	return &MergeResult{Tree: tree, ConflictErr: conflictErr}, nil
}

// Inspect reports any conflict across all files: a Type-1 file conflict
// (*secrets.ConflictError) or a Type-2 env var collision
// (*secrets.EnvConflictError). It returns nil when the whole set is clean.
func (e *Engine) Inspect() error {
	return e.InspectScoped("", false)
}

// InspectScoped is like Inspect but narrows detection to a dot-path and can model
// the --prefixed resolution. Conflicts and collisions outside the scope are
// resolved away so a caller can check a single path in isolation; passing ""
// inspects the whole tree. When prefixed is true, env var names use their full
// dot-path (as `--prefixed` does), so Type-2 collisions cannot occur and only
// Type-1 file conflicts are reported — letting a caller confirm that --prefixed
// resolves the collisions.
func (e *Engine) InspectScoped(scopePrefix string, prefixed bool) error {
	files, err := e.load()
	if err != nil {
		return err
	}
	// Type-1: same dot-path defined in multiple files.
	tree, mergeErr := secrets.Merge(files, config.MergeModeError, scopePrefix)
	if mergeErr != nil {
		return mergeErr
	}
	if prefixed {
		// Full-path env var names never collide; nothing further to check.
		return nil
	}
	// Type-2: distinct dot-paths collapsing to the same env var name.
	if _, envErr := secrets.ToFlatEnvEntries(tree, scopePrefix); envErr != nil {
		return envErr
	}
	return nil
}

// InspectAllResult holds all detected issues across the three error categories.
// Unlike InspectScoped, collection does not short-circuit on the first error type.
type InspectAllResult struct {
	ConflictErr    *secrets.ConflictError
	EnvConflictErr *secrets.EnvConflictError
}

// InspectAll runs all checks without short-circuiting, so every error category
// is always reported even when multiple types are present.
// When prefixed is true, Type-2 env var collisions are not checked (same
// semantics as InspectScoped).
func (e *Engine) InspectAll(scopePrefix string, prefixed bool) (InspectAllResult, error) {
	files, err := e.load()
	if err != nil {
		return InspectAllResult{}, err
	}

	var result InspectAllResult

	// Type-1: same dot-path defined in multiple files.
	// Use override mode to get a full tree even when conflicts exist, then
	// capture the conflict error separately.
	if _, mergeErr := secrets.Merge(files, config.MergeModeError, scopePrefix); mergeErr != nil {
		if ce, ok := mergeErr.(*secrets.ConflictError); ok {
			result.ConflictErr = ce
		} else {
			return InspectAllResult{}, mergeErr
		}
	}

	if prefixed {
		return result, nil
	}

	// Type-2: distinct dot-paths collapsing to the same env var name.
	// Use override merge to always produce a tree for env-var checking.
	tree, err := secrets.Merge(files, config.MergeModeOverride, scopePrefix)
	if err != nil {
		return InspectAllResult{}, err
	}
	if _, envErr := secrets.ToFlatEnvEntries(tree, scopePrefix); envErr != nil {
		if ece, ok := envErr.(*secrets.EnvConflictError); ok {
			result.EnvConflictErr = ece
		} else {
			return InspectAllResult{}, envErr
		}
	}

	return result, nil
}

// EnvVars resolves env vars from the merged result.
// Flat leaf names (DATABASE_URL), or full path if --prefixed.
func (e *Engine) EnvVars(r *MergeResult, prefixed bool) (map[string]secrets.EnvEntry, error) {
	return e.EnvVarsPrefer(r, prefixed, "")
}

// EnvVarsPrefer is like EnvVars but resolves env var collisions in favour of
// entries whose dot-path is under preferPrefix. All other vars from the full
// tree are still included.
func (e *Engine) EnvVarsPrefer(r *MergeResult, prefixed bool, preferPrefix string) (map[string]secrets.EnvEntry, error) {
	if prefixed {
		return secrets.ToEnvEntries(r.Tree), nil
	}
	return secrets.ToFlatEnvEntries(r.Tree, preferPrefix)
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
// location, or a *secrets.KeyNotFoundError (naming the level where the path
// broke and the keys available there) when it does not exist.
func (e *Engine) GetAtPath(r *MergeResult, dotPath string) (*secrets.Node, error) {
	return secrets.Lookup(r.Tree, dotPath)
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

// LoadFiles discovers and loads all .ward files from every configured vault,
// returning them parsed but unmerged (in config/discovery order).
func (e *Engine) LoadFiles() ([]secrets.ParsedFile, error) {
	return e.load()
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


func sourcePaths(cfg *config.Config) []string {
	paths := make([]string, len(cfg.Vaults))
	for i, s := range cfg.Vaults {
		paths[i] = s.Path
	}
	return paths
}

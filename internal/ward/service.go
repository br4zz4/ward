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
	AnchorPath  string
	ConflictErr *secrets.ConflictError // non-nil only when called via MergeForView
}

// Merge loads all .ward files for the given anchor (empty = all sources) and
// merges them using the mode from the configuration.
func (e *Engine) Merge(anchorPath string) (*MergeResult, error) {
	files, err := e.load()
	if err != nil {
		return nil, err
	}
	ordered, err := e.order(anchorPath, files)
	if err != nil {
		return nil, err
	}
	tree, err := secrets.Merge(ordered, e.mergeMode(anchorPath))
	if err != nil {
		return nil, err
	}
	return &MergeResult{Tree: tree, AnchorPath: anchorPath}, nil
}

// MergeForView is like Merge but always produces a complete tree even when
// conflicts exist. Conflict information is attached to the result so the
// presentation layer can highlight conflicting keys.
func (e *Engine) MergeForView(anchorPath string) (*MergeResult, error) {
	files, err := e.load()
	if err != nil {
		return nil, err
	}
	ordered, err := e.order(anchorPath, files)
	if err != nil {
		return nil, err
	}

	// First pass: detect conflicts without blocking.
	var conflictErr *secrets.ConflictError
	if _, cerr := secrets.Merge(ordered, config.MergeModeError); cerr != nil {
		if ce, ok := cerr.(*secrets.ConflictError); ok {
			conflictErr = ce
		} else {
			return nil, cerr
		}
	}

	// Second pass: override mode so we always get a full tree.
	tree, err := secrets.Merge(ordered, config.MergeModeOverride)
	if err != nil {
		return nil, err
	}
	return &MergeResult{Tree: tree, AnchorPath: anchorPath, ConflictErr: conflictErr}, nil
}

// Inspect runs a conflict-only merge and returns a ConflictError if any conflicts
// exist, or nil when the set of files is clean.
func (e *Engine) Inspect(anchorPath string) error {
	files, err := e.load()
	if err != nil {
		return err
	}
	ordered, err := e.order(anchorPath, files)
	if err != nil {
		return err
	}
	_, mergeErr := secrets.Merge(ordered, config.MergeModeError)
	return mergeErr
}

// EnvVars resolves the env vars for a merged result.
// Without an anchor (or with prefixed=true) it returns full dot-path names.
// With an anchor it returns names relative to the anchor's container level.
func (e *Engine) EnvVars(r *MergeResult, prefixed bool) (map[string]secrets.EnvEntry, error) {
	if prefixed || r.AnchorPath == "" {
		return secrets.ToEnvEntries(r.Tree), nil
	}
	anchorData, err := e.anchorData(r.AnchorPath)
	if err != nil {
		return nil, err
	}
	return secrets.ToEnvEntriesFromAnchor(r.Tree, anchorData), nil
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

func (e *Engine) order(anchorPath string, files []secrets.ParsedFile) ([]secrets.ParsedFile, error) {
	if anchorPath == "" {
		return secrets.SortBySpecificity(files), nil
	}
	info, err := os.Stat(anchorPath)
	if err != nil {
		return nil, fmt.Errorf("anchor not found: %s", anchorPath)
	}
	if info.IsDir() {
		return e.orderForDirAnchor(anchorPath, files)
	}
	return e.orderForFileAnchor(anchorPath, files)
}

func (e *Engine) orderForFileAnchor(anchorPath string, files []secrets.ParsedFile) ([]secrets.ParsedFile, error) {
	var anchor secrets.ParsedFile
	for _, f := range files {
		if f.File == anchorPath {
			anchor = f
			break
		}
	}
	if anchor.File == "" {
		return nil, fmt.Errorf("anchor file not in sources: %s", anchorPath)
	}
	return secrets.FilterByAnchor(anchor, files), nil
}

func (e *Engine) orderForDirAnchor(anchorPath string, files []secrets.ParsedFile) ([]secrets.ParsedFile, error) {
	dirPaths, err := secrets.Discover([]string{anchorPath})
	if err != nil {
		return nil, err
	}
	dirSet := make(map[string]bool, len(dirPaths))
	for _, p := range dirPaths {
		dirSet[p] = true
	}

	var dirFiles []secrets.ParsedFile
	for _, f := range files {
		if dirSet[f.File] {
			dirFiles = append(dirFiles, f)
		}
	}

	seen := map[string]bool{}
	var ancestors []secrets.ParsedFile
	for _, df := range dirFiles {
		for _, f := range files {
			if dirSet[f.File] || seen[f.File] {
				continue
			}
			if secrets.IsAncestorOf(f, df) && secrets.MapDepth(f.Data) < secrets.MapDepth(df.Data) {
				seen[f.File] = true
				ancestors = append(ancestors, secrets.TrimToScope(f, dirFiles))
			}
		}
	}

	return secrets.SortBySpecificity(append(ancestors, dirFiles...)), nil
}

// mergeMode returns the effective merge mode for a given anchor.
// Dir anchors always use MergeModeError — sibling conflicts are always ambiguous.
// File anchors and no-anchor use the configured mode.
func (e *Engine) mergeMode(anchorPath string) config.MergeMode {
	if anchorPath == "" {
		return e.cfg.Merge
	}
	if info, err := os.Stat(anchorPath); err == nil && info.IsDir() {
		return config.MergeModeError
	}
	return e.cfg.Merge
}

// anchorData loads the YAML structure from an anchor path (file or first file in
// dir). Used to determine the container level for relative env var naming.
func (e *Engine) anchorData(anchorPath string) (map[string]interface{}, error) {
	info, err := os.Stat(anchorPath)
	if err != nil {
		return nil, fmt.Errorf("anchor not found: %s", anchorPath)
	}
	target := anchorPath
	if info.IsDir() {
		paths, err := secrets.Discover([]string{anchorPath})
		if err != nil || len(paths) == 0 {
			return nil, fmt.Errorf("no .ward files in anchor dir: %s", anchorPath)
		}
		target = paths[0]
	}
	pf, err := secrets.Load(target, e.dec)
	if err != nil {
		return nil, err
	}
	return pf.Data, nil
}

func sourcePaths(cfg *config.Config) []string {
	paths := make([]string, len(cfg.Sources))
	for i, s := range cfg.Sources {
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

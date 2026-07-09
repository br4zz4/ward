package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/ward"
)

// secretEditor owns the shared pipeline behind `set` and `unset`: resolve the
// project, the vault and the target file, then decrypt → mutate → encrypt. The
// parts that differ between the two commands are supplied by a mutation.
type secretEditor struct {
	eng   *ward.Engine
	cfg   *config.Config
	cfgPath string
	files []secrets.ParsedFile
}

// newSecretEditor builds the editor, validating the project up front. It exits
// (via fatal*) on any setup failure so callers get a ready-to-use editor.
func newSecretEditor() *secretEditor {
	enforceVaultStructure()

	cfgPath, err := resolvedConfigFile()
	if err != nil {
		fatal(fmt.Errorf("no ward project found — run `ward init` first"))
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatal(err)
	}
	eng, err := newEngine()
	if err != nil {
		fatal(err)
	}
	files, err := eng.LoadFiles()
	if err != nil {
		fatal(err)
	}
	return &secretEditor{eng: eng, cfg: cfg, cfgPath: cfgPath, files: files}
}

// vaultFor resolves the vault named by the dot-path's first segment, exiting
// with a styled error when it does not exist.
func (e *secretEditor) vaultFor(dotPath string) *config.Source {
	name := firstSegment(dotPath)
	src := findVault(e.cfg, name)
	if src == nil {
		fatalVaultNotFound(name)
	}
	return src
}

// abortOnAmbiguity exits when the dot-path is defined in more than one file
// (a Type-1 conflict: ward cannot know which file to touch).
func (e *secretEditor) abortOnAmbiguity(dotPath string, targets []string) {
	if len(targets) > 1 {
		fatal(fmt.Errorf("%s", ambiguousTargetError(dotPath, targets, e.files)))
	}
}

// keyNotFound builds a level-aware "key not found" error from the loaded files:
// it merges them (ignoring conflicts) and reports which keys were available at
// the level where dotPath broke. Falls back to a plain message if the merge fails.
func (e *secretEditor) keyNotFound(dotPath string) error {
	tree, err := secrets.Merge(e.files, config.MergeModeOverride, "")
	if err != nil {
		return fmt.Errorf("key not found: %s", dotPath)
	}
	_, lookupErr := secrets.Lookup(tree, dotPath)
	if lookupErr != nil {
		return lookupErr
	}
	return fmt.Errorf("key not found: %s", dotPath)
}

// load decrypts and parses targetPath into a mutable Tree.
func (e *secretEditor) load(targetPath string) *secrets.Tree {
	plain, err := e.eng.Decrypt(targetPath)
	if err != nil {
		fatal(fmt.Errorf("decrypting %s: %w", targetPath, err))
	}
	data := map[string]interface{}{}
	if err := yaml.Unmarshal(plain, &data); err != nil {
		fatal(fmt.Errorf("parsing %s: %w", targetPath, err))
	}
	return secrets.NewTree(data)
}

// save marshals the tree and re-encrypts it to targetPath, creating the parent
// directory when needed.
func (e *secretEditor) save(targetPath string, tree *secrets.Tree) {
	out, err := yaml.Marshal(tree.Root())
	if err != nil {
		fatal(fmt.Errorf("encoding YAML: %w", err))
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		fatal(fmt.Errorf("creating directory: %w", err))
	}
	if err := e.eng.Encrypt(targetPath, out); err != nil {
		fatal(fmt.Errorf("encrypting %s: %w", targetPath, err))
	}
}

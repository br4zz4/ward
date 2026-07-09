package cmd

import (
	"fmt"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/ward"
)

// stampEnvCommand tags a Type-2 collision error with the command that surfaced
// it, so its resolution examples show that command (envs/exec/inspect). Any
// other error passes through unchanged. Returns the (possibly-stamped) error.
func stampEnvCommand(err error, command string) error {
	if ce, ok := err.(*secrets.EnvConflictError); ok {
		ce.Cmd = command
	}
	return err
}

// ambiguousTargetError builds a Type-1 conflict message: the dot-path is defined
// in more than one file, so ward cannot know which file to write.
func ambiguousTargetError(dotPath string, targets []string, files []secrets.ParsedFile) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s%scannot set %s%s%s — defined in %d files:%s\n\n",
		clrBold, clrLightRed, clrReset+clrBold, dotPath, clrReset+clrBold+clrLightRed, len(targets), clrReset)
	for _, t := range targets {
		if line := lineFor(files, t, dotPath); line > 0 {
			fmt.Fprintf(&sb, "  %s%s%s:%s%d%s\n", clrCyan, t, clrReset, clrLightRed, line, clrReset)
		} else {
			fmt.Fprintf(&sb, "  %s%s%s\n", clrCyan, t, clrReset)
		}
	}
	fmt.Fprintf(&sb, "\n  %sto resolve:%s keep %s%s%s in only one of the files above\n",
		clrBold, clrReset, clrYellow, secrets.LeafKey(dotPath), clrReset)
	return sb.String()
}

// lineFor returns the source line of dotPath in the given file, or 0.
func lineFor(files []secrets.ParsedFile, file, dotPath string) int {
	for _, pf := range files {
		if pf.File == file {
			return pf.Lines[dotPath]
		}
	}
	return 0
}

// envCollisionWarning re-merges after a write and returns a non-blocking warning
// if the touched dot-path now leaves its env var in a Type-2 collision. Empty
// string means no warning.
func envCollisionWarning(eng *ward.Engine, dotPath string) string {
	result, err := eng.MergeForView()
	if err != nil {
		return ""
	}
	collision := envCollisionFor(result.Tree, dotPath)
	if collision == nil {
		return ""
	}
	return formatCollisionWarning(collision)
}

// formatCollisionWarning renders the styled warning body for a Type-2 collision.
func formatCollisionWarning(collision *secrets.EnvConflict) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  %s⚠ warning:%s env var %s%s%s now collides\n",
		clrYellow+clrBold, clrReset, clrBold, collision.EnvKey, clrReset)
	if collision.CaseCollision {
		fmt.Fprintf(&sb, "    %s%s%s and %s%s%s differ only in case\n",
			clrYellow, collision.DotPaths[0], clrReset, clrYellow, collision.DotPaths[1], clrReset)
		fmt.Fprintf(&sb, "    %s→%s use consistent casing across vaults\n\n", clrGray, clrReset)
		return sb.String()
	}
	fmt.Fprintf(&sb, "    %s%s%s\n", clrYellow, collision.DotPaths[0], clrReset)
	fmt.Fprintf(&sb, "    %s%s%s\n", clrYellow, collision.DotPaths[1], clrReset)
	fmt.Fprintf(&sb, "    %s→%s use %s--prefixed%s or scope the path on %sward exec%s\n\n",
		clrGray, clrReset, clrCyan, clrReset, clrCyan, clrReset)
	return sb.String()
}

// envCollisionFor returns the Type-2 env var collision affecting dotPath's leaf,
// or nil if there is none.
func envCollisionFor(tree map[string]*secrets.Node, dotPath string) *secrets.EnvConflict {
	_, err := secrets.ToFlatEnvEntries(tree, "")
	if err == nil {
		return nil
	}
	ce, ok := err.(*secrets.EnvConflictError)
	if !ok {
		return nil
	}
	leaf := strings.ReplaceAll(secrets.LeafKey(dotPath), "-", "_")
	for i := range ce.Conflicts {
		if collisionMatchesLeaf(ce.Conflicts[i], leaf) {
			return &ce.Conflicts[i]
		}
	}
	return nil
}

// collisionMatchesLeaf reports whether a collision concerns the given env leaf,
// treating case-only collisions case-insensitively.
func collisionMatchesLeaf(c secrets.EnvConflict, leaf string) bool {
	if c.CaseCollision {
		return strings.EqualFold(c.EnvKey, leaf)
	}
	return c.EnvKey == leaf
}

package cmd

import (
	"github.com/spf13/cobra"
)

// NewOverrideCmd is the deprecated alias of `import`. It still works but is
// hidden from help and prints a deprecation notice after running.
func NewOverrideCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "override <file.ward>",
		Short:  "Deprecated: use 'import'",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		Run: func(_ *cobra.Command, args []string) {
			runImport(args[0])
			warnDeprecated("override", "import")
		},
	}
}

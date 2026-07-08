package cmd

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/br4zz4/ward/internal/config"
	"github.com/spf13/cobra"
)

func NewUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "unset <dot.path>",
		Short:             "Remove a single secret at a full dot-path",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath := args[0]

			enforceVaultStructure()

			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("no ward project found — run `ward init` first"))
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}

			vaultName := firstSegment(dotPath)
			if findVault(cfg, vaultName) == nil {
				fatalVaultNotFound(vaultName)
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			files, err := eng.LoadFiles()
			if err != nil {
				fatal(err)
			}

			// Type-1 conflict: same dot-path defined in more than one file → abort.
			targets := resolveTargetFiles(files, dotPath)
			if len(targets) > 1 {
				fatal(fmt.Errorf("%s", ambiguousTargetError(dotPath, targets, files)))
			}
			if len(targets) == 0 {
				fatal(fmt.Errorf("key not found: %s", dotPath))
			}

			targetPath := targets[0]
			plain, err := eng.Decrypt(targetPath)
			if err != nil {
				fatal(fmt.Errorf("decrypting %s: %w", targetPath, err))
			}
			data := map[string]interface{}{}
			if err := yaml.Unmarshal(plain, &data); err != nil {
				fatal(fmt.Errorf("parsing %s: %w", targetPath, err))
			}

			if !unsetLeaf(data, dotPath) {
				fatal(fmt.Errorf("key not found: %s", dotPath))
			}

			out, err := yaml.Marshal(data)
			if err != nil {
				fatal(fmt.Errorf("encoding YAML: %w", err))
			}
			if err := eng.Encrypt(targetPath, out); err != nil {
				fatal(fmt.Errorf("encrypting %s: %w", targetPath, err))
			}

			fmt.Fprintf(os.Stderr, "  %s✓%s unset %s%s%s\n", clrGreen, clrReset, clrBold, dotPath, clrReset)
		},
	}
}

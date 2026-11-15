package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	wardage "github.com/br4zz4/ward/internal/age"
	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage vaults",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newVaultListCmd())
	cmd.AddCommand(newVaultAddCmd())
	return cmd
}

func newVaultListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured vaults",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}
			if len(cfg.Vaults) == 0 {
				fmt.Printf("\n  %s✗ no vaults configured%s — add a vault to %s%s%s\n\n",
					clrLightRed, clrReset, clrCyan, cfgPath, clrReset)
				return
			}
			ruler := func(label string) {
				dashes := 52 - len(label) - 1
				fmt.Printf("\n  %s%s%s%s%s\n",
					clrCyan+clrBold, label, clrReset+clrGray, strings.Repeat("─", dashes), clrReset)
			}
			ruler("VAULTS")
			for _, v := range cfg.Vaults {
				fmt.Printf("  %s●%s %s%s%s  %s%s%s\n",
					clrGreen, clrReset,
					clrCyan, v.Name, clrReset,
					clrGrayLight, v.Path, clrReset)
			}
			fmt.Printf("\n  %s%d vault(s) configured in %s%s\n\n",
				clrGray, len(cfg.Vaults), cfgPath, clrReset)
		},
	}
}

func newVaultAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> [path]",
		Short: "Register a new vault (generates its key when the path has no secrets yet)",
		Long: `Register a new vault. The path defaults to .ward/vaults/<name>.

When the path has no .ward files yet, ward also generates the vault's key at
.ward/<name>.key (gitignored), creates the directory with an encrypted
secrets.ward and prints the key token for CI. When the path already holds .ward
files, the vault is only registered.

Sub-vaults use <namespace>::<name>, e.g. ward vault add myfish::restricted`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			name := args[0]
			vaultPath := ".ward/vaults/" + name
			if len(args) == 2 {
				vaultPath = args[1]
			}

			if err := config.ValidateVaultName(name); err != nil {
				fatal(err)
			}

			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("no ward project found — run `ward init` first"))
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}

			for _, v := range cfg.Vaults {
				if v.Name == name || v.CanonicalName() == config.CanonicalVaultName(name) {
					fatal(fmt.Errorf("vault %q already exists", name))
				}
				if strings.TrimSuffix(v.Path, "/") == strings.TrimSuffix(vaultPath, "/") {
					fatal(fmt.Errorf("path %q is already used by vault %q", vaultPath, v.Name))
				}
			}

			next := append(append([]config.Source{}, cfg.Vaults...), config.Source{Name: name, Path: vaultPath})
			if err := config.CheckVaultIdentities(next); err != nil {
				fatal(err)
			}

			existing, err := secrets.Discover([]string{vaultPath})
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				fatal(fmt.Errorf("reading %s: %w", vaultPath, err))
			}
			var token string
			var cleanup func()
			if len(existing) == 0 {
				token, cleanup, err = provisionVault(name, vaultPath)
				if err != nil {
					fatal(err)
				}
			}

			cfg.Vaults = append(cfg.Vaults, config.Source{Name: name, Path: vaultPath})
			if err := config.Save(cfgPath, cfg); err != nil {
				if cleanup != nil {
					cleanup()
				}
				fatal(fmt.Errorf("updating %s: %w", cfgPath, err))
			}
			fmt.Printf("\n  %s✓%s vault %s%s%s added (%s%s%s)\n\n",
				clrGreen, clrReset,
				clrCyan, name, clrReset,
				clrGrayLight, vaultPath, clrReset)
			if token != "" {
				fmt.Printf("  %s=%s\n", config.KeyEnvName(name), token)
				fmt.Printf("  %s↑ copy this to CI / secrets manager — key stored in .ward/%s.key%s\n\n", clrGray, name, clrReset)
			}
		},
	}
}

// provisionVault generates .ward/<name>.key, gitignores it, creates the vault
// directory with an encrypted secrets.ward rooted at name, and returns the
// portable key token plus a cleanup that undoes what it created.
func provisionVault(name, vaultPath string) (token string, cleanup func(), err error) {
	keyFile := ".ward/" + name + ".key"
	if _, statErr := os.Stat(keyFile); statErr == nil {
		return "", nil, fmt.Errorf("%s already exists — remove it or pick another name", keyFile)
	}
	if err := os.MkdirAll(".ward", 0755); err != nil {
		return "", nil, fmt.Errorf("creating .ward/: %w", err)
	}
	if err := ensureGitignore(keyFile); err != nil {
		return "", nil, err
	}
	dirExisted := true
	if _, statErr := os.Stat(vaultPath); statErr != nil {
		dirExisted = false
	}
	cleanup = func() {
		os.Remove(keyFile)
		if !dirExisted {
			os.RemoveAll(vaultPath)
		} else {
			os.Remove(vaultPath + "/secrets.ward")
		}
	}
	fail := func(e error) (string, func(), error) {
		cleanup()
		return "", nil, e
	}
	if err := wardage.GenerateKey(keyFile); err != nil {
		return fail(err)
	}
	token, err = encodeWardKey(keyFile)
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(vaultPath, 0755); err != nil {
		return fail(fmt.Errorf("creating %s/: %w", vaultPath, err))
	}
	if err := encryptIfAbsent(vaultPath+"/secrets.ward", initSecretsStub(name), keyFile); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(keyFile, []byte(token+"\n"), 0600); err != nil {
		return fail(fmt.Errorf("writing token to %s: %w", keyFile, err))
	}
	return token, cleanup, nil
}

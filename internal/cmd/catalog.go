package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/ward"
	"github.com/spf13/cobra"
)

// catalogEntry is one row of the catalog: a secret path with its optional
// metadata (description, type) lifted from the `_meta` block of the file that
// defines it. Values are never part of a catalog entry.
type catalogEntry struct {
	path        string
	description string
	typeName    string
}

func NewCatalogCmd() *cobra.Command {
	var jsonFlag bool
	c := &cobra.Command{
		Use:   "catalog [--json]",
		Short: "List secret paths with descriptions (values never shown)",
		Args:  cobra.NoArgs,
		Run:   func(c *cobra.Command, _ []string) { runCatalog(c, jsonFlag) },
	}
	c.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON (for machine consumption)")
	return c
}

// runCatalog lists every leaf path of the merged tree together with the
// metadata (description/type) documented in the defining file's `_meta` block.
// It never prints secret values — it is safe to expose to AI agents.
func runCatalog(c *cobra.Command, asJSON bool) {
	enforceVaultStructure()
	eng, err := newEngine()
	if err != nil {
		fatal(err)
	}
	result, err := eng.MergeScoped("")
	if err != nil {
		fatal(err)
	}
	printEngineWarnings(eng)

	metaByFile := collectFileMeta(eng)
	entries := buildCatalog(result.Tree, metaByFile)

	if asJSON {
		rows := make([]map[string]string, 0, len(entries))
		for _, e := range entries {
			rows = append(rows, map[string]string{
				"path":        e.path,
				"description": e.description,
				"type":        e.typeName,
			})
		}
		out, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			fatal(fmt.Errorf("encoding JSON: %w", err))
		}
		fmt.Println(string(out))
		return
	}
	printCatalogTable(entries)
}

// collectFileMeta returns a map of dot-path → metadata map, gathered from every
// loaded file's `_meta` block. A path documented in several files keeps the
// metadata from the last (most specific) file, mirroring override merge order.
func collectFileMeta(eng *ward.Engine) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	files, err := eng.LoadFiles()
	if err != nil {
		return out
	}
	for _, pf := range files {
		for dotPath, raw := range pf.Meta {
			if m, ok := raw.(map[string]interface{}); ok {
				out[dotPath] = m
			}
		}
	}
	return out
}

// buildCatalog walks the merged tree and produces one entry per leaf path,
// sorted by path. Metadata is looked up by dot-path.
func buildCatalog(tree map[string]*secrets.Node, metaByFile map[string]map[string]interface{}) []catalogEntry {
	var entries []catalogEntry
	collectCatalogLeaves(tree, "", metaByFile, &entries)
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	return entries
}

// collectCatalogLeaves walks nodes depth-first, emitting one catalogEntry per leaf.
func collectCatalogLeaves(nodes map[string]*secrets.Node, prefix string, metaByFile map[string]map[string]interface{}, out *[]catalogEntry) {
	for k, node := range nodes {
		dotPath := k
		if prefix != "" {
			dotPath = prefix + "." + k
		}
		if node.Children != nil {
			collectCatalogLeaves(node.Children, dotPath, metaByFile, out)
			continue
		}
		e := catalogEntry{path: dotPath}
		if m, ok := metaByFile[dotPath]; ok {
			if d, ok := m["description"].(string); ok {
				e.description = d
			}
			if ty, ok := m["type"].(string); ok {
				e.typeName = ty
			}
		}
		*out = append(*out, e)
	}
}

// printCatalogTable renders the catalog as an aligned table: path | type | description.
func printCatalogTable(entries []catalogEntry) {
	maxPath, maxType := 0, 0
	for _, e := range entries {
		if len([]rune(e.path)) > maxPath {
			maxPath = len([]rune(e.path))
		}
		if len([]rune(e.typeName)) > maxType {
			maxType = len([]rune(e.typeName))
		}
	}
	for _, e := range entries {
		pathPad := strings.Repeat(" ", maxPath-len([]rune(e.path)))
		typePad := strings.Repeat(" ", maxType-len([]rune(e.typeName)))
		fmt.Printf("%s%s%s%s  %s%s%s%s  %s\n",
			clrCyan, e.path, clrReset, pathPad,
			clrGray, e.typeName, clrReset, typePad,
			e.description)
	}
}
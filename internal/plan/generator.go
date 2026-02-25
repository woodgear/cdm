// Package plan provides plan generation functionality
package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodgear/cdm/internal/config"
	"github.com/woodgear/cdm/internal/fs"
	"github.com/woodgear/cdm/pkg/types"
)

// Scanner scans directories for config files
type Scanner struct {
	verbose bool
}

// NewScanner creates a new scanner
func NewScanner(verbose bool) *Scanner {
	return &Scanner{verbose: verbose}
}

// ScanDir scans a directory for files to link
// baseType: "home" maps to $HOME, "root" maps to /
func (s *Scanner) ScanDir(srcDir, baseType string) ([]types.FileEntry, error) {
	var entries []types.FileEntry

	var basePath string
	switch baseType {
	case "home":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		basePath = home
	case "root":
		basePath = ""
	default:
		return nil, fmt.Errorf("invalid base type: %s", baseType)
	}

	scanPath := filepath.Join(srcDir, baseType)

	info, err := os.Stat(scanPath)
	if err != nil {
		if os.IsNotExist(err) {
			if s.verbose {
				fmt.Printf("[SKIP] Directory not found: %s\n", scanPath)
			}
			return entries, nil
		}
		return nil, fmt.Errorf("failed to stat %s: %w", scanPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", scanPath)
	}

	if s.verbose {
		fmt.Printf("[SCAN] %s\n", scanPath)
	}

	// Walk the directory tree
	err = filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(scanPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Build target path
		var targetPath string
		if basePath == "" {
			targetPath = filepath.Join("/", relPath)
		} else {
			targetPath = filepath.Join(basePath, relPath)
		}

		// Get absolute source path
		absSource, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		entries = append(entries, types.FileEntry{
			Source:     absSource,
			Target:     targetPath,
			SourcePath: srcDir,
			Reason:     "new",
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", scanPath, err)
	}

	return entries, nil
}

// Generator generates execution plans
type Generator struct {
	verbose      bool
	scanner      *Scanner
	configLoader *config.Loader
}

// NewGenerator creates a new plan generator
func NewGenerator(verbose bool) *Generator {
	return &Generator{
		verbose:      verbose,
		scanner:      NewScanner(verbose),
		configLoader: config.NewLoader(),
	}
}

// Generate generates an execution plan from source paths
func (g *Generator) Generate(sourcePaths []string) (*types.Plan, error) {
	if g.verbose {
		fmt.Printf("[INFO] Generating execution plan...\n")
		fmt.Printf("[INFO] Sources: %s\n", strings.Join(sourcePaths, " "))
	}

	// Validate and resolve source paths
	var resolvedPaths []string
	for _, path := range sourcePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("source path does not exist: %s", absPath)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source path is not a directory: %s", absPath)
		}

		resolvedPaths = append(resolvedPaths, absPath)
	}

	// Scan all source directories
	var allEntries []types.FileEntry
	for _, srcPath := range resolvedPaths {
		if g.verbose {
			fmt.Printf("[INFO] Processing: %s\n", srcPath)
		}

		// Scan home directory
		homeEntries, err := g.scanner.ScanDir(srcPath, "home")
		if err != nil {
			return nil, fmt.Errorf("failed to scan home directory in %s: %w", srcPath, err)
		}
		allEntries = append(allEntries, homeEntries...)

		// Scan root directory
		rootEntries, err := g.scanner.ScanDir(srcPath, "root")
		if err != nil {
			return nil, fmt.Errorf("failed to scan root directory in %s: %w", srcPath, err)
		}
		allEntries = append(allEntries, rootEntries...)
	}

	// Remove duplicates and mark overrides (later sources override earlier ones)
	targetMap := make(map[string]types.FileEntry)
	for _, entry := range allEntries {
		if existing, ok := targetMap[entry.Target]; ok {
			// Override - update reason
			existing.Reason = fmt.Sprintf("override from %s", filepath.Base(entry.SourcePath))
			existing.Source = entry.Source
			existing.SourcePath = entry.SourcePath
			targetMap[entry.Target] = existing
			if g.verbose {
				fmt.Printf("[OVERRIDE] %s\n", entry.Target)
			}
		} else {
			targetMap[entry.Target] = entry
			if g.verbose {
				fmt.Printf("[NEW] %s\n", entry.Target)
			}
		}
	}

	// Convert map to slice
	entries := make([]types.FileEntry, 0, len(targetMap))
	for _, entry := range targetMap {
		entries = append(entries, entry)
	}

	// Load and apply configurations
	configs, err := g.configLoader.LoadAll(resolvedPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load configurations: %w", err)
	}
	entries = g.applyPathMappings(configs, entries)

	// Build links
	var statNew, statOverride int
	links := make([]types.Link, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Reason, "override") {
			statOverride++
		} else {
			statNew++
		}

		links = append(links, types.Link{
			Source: entry.Source,
			Target: entry.Target,
			Action: "link",
			Reason: entry.Reason,
		})
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Build plan
	plan := &types.Plan{
		Version:   "1.0.0",
		Timestamp: time.Now(),
		Hostname:  hostname,
		Sources:   resolvedPaths,
		Links:     links,
		Stats: types.Stats{
			Total:    len(links),
			New:      statNew,
			Override: statOverride,
			Skip:     0,
		},
	}

	return plan, nil
}

// applyPathMappings applies path mappings from configuration files
func (g *Generator) applyPathMappings(configs map[string]*types.Config, entries []types.FileEntry) []types.FileEntry {
	home, _ := os.UserHomeDir()

	result := make([]types.FileEntry, len(entries))
	copy(result, entries)

	for srcPath, cfg := range configs {
		if cfg.PathMappings == nil || len(cfg.PathMappings) == 0 {
			continue
		}

		for i, entry := range result {
			for _, mapping := range cfg.PathMappings {
				// Get relative path from home
				var relPath string
				if home != "" && strings.HasPrefix(entry.Target, home) {
					relPath = strings.TrimPrefix(entry.Target, home)
					relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
				} else if strings.HasPrefix(entry.Target, "/") {
					relPath = strings.TrimPrefix(entry.Target, "/")
				}

				// Check if relPath starts with mapping source
				if strings.HasPrefix(relPath, mapping.Source) {
					// Calculate new target
					newTarget := mapping.Target + strings.TrimPrefix(relPath, mapping.Source)

					// Expand ~ in target
					expanded, err := fs.ExpandPath(newTarget)
					if err != nil {
						continue
					}

					result[i].Target = expanded
					result[i].Reason = fmt.Sprintf("%s (remapped by %s)", entry.Reason, filepath.Base(srcPath))

					if g.verbose {
						fmt.Printf("[REMAP] %s -> %s\n", entry.Target, expanded)
					}
				}
			}
		}
	}

	return result
}

// Package plan provides plan generation functionality
package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/woodgear/cdm/internal/config"
	"github.com/woodgear/cdm/internal/fs"
	"github.com/woodgear/cdm/internal/version"
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
// linkFolders: set of absolute paths that should be linked as folders
func (s *Scanner) ScanDir(srcDir, baseType string, linkFolders map[string]bool, skipPaths map[string]bool) ([]types.FileEntry, error) {
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

		for skipPath, skipDir := range skipPaths {
			if absSource == skipPath || (skipDir && strings.HasPrefix(absSource, skipPath+string(filepath.Separator))) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check if this path is under a linkFolder
		for folderPath := range linkFolders {
			// If this path is a linkFolder itself
			if absSource == folderPath && info.IsDir() {
				// Add folder link entry
				entries = append(entries, types.FileEntry{
					Source:     absSource,
					Target:     targetPath,
					SourcePath: srcDir,
					Action:     types.ActionLink,
					Reason:     "folder link",
				})
				if s.verbose {
					fmt.Printf("[FOLDER_LINK] %s -> %s\n", absSource, targetPath)
				}
				// Skip walking into this directory
				return filepath.SkipDir
			}

			// If this path is under a linkFolder
			if strings.HasPrefix(absSource, folderPath+string(filepath.Separator)) {
				// Skip this file/directory
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip directories (they're handled via linkFolders or files inside)
		if info.IsDir() {
			return nil
		}

		entries = append(entries, types.FileEntry{
			Source:     absSource,
			Target:     targetPath,
			SourcePath: srcDir,
			Action:     types.ActionLink,
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
			return nil, fmt.Errorf("failed to stat source path %s: %w", absPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source path is not a directory: %s", absPath)
		}

		resolvedPaths = append(resolvedPaths, absPath)
	}

	// Load configurations first (to get linkFolders)
	configs, err := g.configLoader.LoadAll(resolvedPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load configurations: %w", err)
	}

	// Build linkFolders set from all configs
	linkFolders := make(map[string]bool)
	for _, configPath := range sortedConfigPaths(configs) {
		cfg := configs[configPath]
		for _, folder := range cfg.LinkFolders {
			// Resolve folder path relative to config location
			folderAbsPath := filepath.Join(configPath, folder)
			linkFolders[folderAbsPath] = true
			if g.verbose {
				fmt.Printf("[LINK_FOLDER] %s\n", folderAbsPath)
			}
		}
	}

	// Collect all repos from configs
	var allRepos []types.RepoConfig
	for _, configPath := range sortedConfigPaths(configs) {
		cfg := configs[configPath]
		for _, repo := range cfg.Repos {
			// Resolve repo path relative to config location
			resolvedRepo := repo
			if !filepath.IsAbs(repo.Path) {
				resolvedRepo.Path = filepath.Join(configPath, repo.Path)
			}
			allRepos = append(allRepos, resolvedRepo)
			if g.verbose {
				fmt.Printf("[REPO] %s -> %s (%s)\n", resolvedRepo.Path, repo.URL, repo.Branch)
			}
		}
	}

	copyEntries, copySkipPaths, err := g.collectCopyMappings(configs)
	if err != nil {
		return nil, err
	}

	// Scan all source directories
	var allEntries []types.FileEntry
	for _, srcPath := range resolvedPaths {
		if g.verbose {
			fmt.Printf("[INFO] Processing: %s\n", srcPath)
		}

		// Scan home directory
		homeEntries, err := g.scanner.ScanDir(srcPath, "home", linkFolders, copySkipPaths)
		if err != nil {
			return nil, fmt.Errorf("failed to scan home directory in %s: %w", srcPath, err)
		}
		allEntries = append(allEntries, homeEntries...)

		// Scan root directory
		rootEntries, err := g.scanner.ScanDir(srcPath, "root", linkFolders, copySkipPaths)
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

	// Apply remaps
	entries, err = g.applyRemaps(configs, entries)
	if err != nil {
		return nil, err
	}

	// Collect external links and copy tasks
	externalEntries, err := g.collectExternalLinks(configs)
	if err != nil {
		return nil, err
	}
	entries = append(entries, externalEntries...)
	entries = append(entries, copyEntries...)
	sortEntries(entries)

	targets := make(map[string]types.FileEntry)
	for _, entry := range entries {
		if existing, ok := targets[entry.Target]; ok {
			return nil, fmt.Errorf("target conflict: %s used by %s (%s) and %s (%s)",
				entry.Target, existing.Source, existing.Action, entry.Source, entry.Action)
		}
		targets[entry.Target] = entry
	}

	// Build tasks
	var statOverride int
	tasks := make([]types.Task, 0, len(entries))
	actionStats := make(map[string]int)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Reason, "override") {
			statOverride++
		}

		action := entry.Action
		if action == "" {
			action = types.ActionLink
		}
		actionStats[action]++

		tasks = append(tasks, types.Task{
			Source: entry.Source,
			Target: entry.Target,
			Action: action,
			Reason: entry.Reason,
		})
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// Build plan
	plan := &types.Plan{
		Version:   version.Current().Version,
		Timestamp: time.Now(),
		Hostname:  hostname,
		Sources:   resolvedPaths,
		Tasks:     tasks,
		Repos:     allRepos,
		Stats: types.Stats{
			Total:          len(tasks),
			Link:           actionStats[types.ActionLink],
			CopyIfNotExist: actionStats[types.ActionCopyIfNotExist],
			Copy:           actionStats[types.ActionCopy],
			Override:       statOverride,
			Skip:           0,
		},
	}

	return plan, nil
}

// applyRemaps applies target remaps from configuration files
func (g *Generator) applyRemaps(configs map[string]*types.Config, entries []types.FileEntry) ([]types.FileEntry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	result := make([]types.FileEntry, len(entries))
	copy(result, entries)

	for _, srcPath := range sortedConfigPaths(configs) {
		cfg := configs[srcPath]
		if cfg.Remaps == nil || len(cfg.Remaps) == 0 {
			continue
		}

		for i, entry := range result {
			for _, mapping := range cfg.Remaps {
				// Get relative path from home
				var relPath string
				if homeRelPath, ok := trimPathPrefix(entry.Target, home); ok {
					relPath = homeRelPath
				} else if strings.HasPrefix(entry.Target, "/") {
					relPath = strings.TrimPrefix(entry.Target, "/")
				}

				// Expand ~ in source and convert to relative path
				sourceExpanded, err := fs.ExpandPath(mapping.Source)
				if err != nil {
					return nil, err
				}
				var sourceRelPath string
				if homeRelPath, ok := trimPathPrefix(sourceExpanded, home); ok {
					sourceRelPath = homeRelPath
				} else if strings.HasPrefix(sourceExpanded, "/") {
					sourceRelPath = strings.TrimPrefix(sourceExpanded, "/")
				} else {
					// Relative path, use as-is
					sourceRelPath = sourceExpanded
				}

				if sourceRelPath == "" {
					return nil, fmt.Errorf("empty remap source in %s", srcPath)
				}

				if pathMatchesOrContains(relPath, sourceRelPath) {
					// Calculate new target
					newTarget := remappedTarget(mapping.Target, strings.TrimPrefix(filepath.Clean(relPath), filepath.Clean(sourceRelPath)))

					// Expand ~ in target
					expanded, err := fs.ExpandPath(newTarget)
					if err != nil {
						return nil, err
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

	return result, nil
}

// collectExternalLinks collects links for files/dirs outside cdm management
func (g *Generator) collectExternalLinks(configs map[string]*types.Config) ([]types.FileEntry, error) {
	var entries []types.FileEntry

	for _, srcPath := range sortedConfigPaths(configs) {
		cfg := configs[srcPath]
		if len(cfg.ExternalLinks) == 0 {
			continue
		}

		for _, mapping := range cfg.ExternalLinks {
			// Expand source path
			sourceExpanded, err := fs.ExpandPath(mapping.Source)
			if err != nil {
				return nil, err
			}

			// Check if source exists on the system
			if _, err := os.Stat(sourceExpanded); err != nil {
				return nil, fmt.Errorf("failed to stat external link source %s: %w", sourceExpanded, err)
			}

			// Expand target path
			targetExpanded, err := fs.ExpandPath(mapping.Target)
			if err != nil {
				return nil, err
			}

			// Create entry: target -> source (symlink points from target to source)
			entry := types.FileEntry{
				Source:     sourceExpanded,
				Target:     targetExpanded,
				SourcePath: srcPath,
				Action:     types.ActionLink,
				Reason:     "external mapping",
			}

			entries = append(entries, entry)

			if g.verbose {
				fmt.Printf("[EXTERNAL_MAPPING] %s -> %s\n", targetExpanded, sourceExpanded)
			}
		}
	}

	return entries, nil
}

func (g *Generator) collectCopyMappings(configs map[string]*types.Config) ([]types.FileEntry, map[string]bool, error) {
	var entries []types.FileEntry
	skipPaths := make(map[string]bool)

	for _, srcPath := range sortedConfigPaths(configs) {
		cfg := configs[srcPath]
		for _, mapping := range cfg.CopyIfNotExist {
			entry, sourceIsDir, err := g.copyEntry(srcPath, mapping, types.ActionCopyIfNotExist)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, entry)
			skipPaths[entry.Source] = sourceIsDir
		}

		for _, mapping := range cfg.Copy {
			entry, sourceIsDir, err := g.copyEntry(srcPath, mapping, types.ActionCopy)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, entry)
			skipPaths[entry.Source] = sourceIsDir
		}
	}

	return entries, skipPaths, nil
}

func (g *Generator) copyEntry(configPath string, mapping types.PathMapping, action string) (types.FileEntry, bool, error) {
	source, err := resolveConfigSource(configPath, mapping.Source)
	if err != nil {
		return types.FileEntry{}, false, err
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return types.FileEntry{}, false, fmt.Errorf("failed to stat copy source %s: %w", source, err)
	}
	target, err := fs.ExpandPath(mapping.Target)
	if err != nil {
		return types.FileEntry{}, false, err
	}
	return types.FileEntry{
		Source:     source,
		Target:     target,
		SourcePath: configPath,
		Action:     action,
		Reason:     action,
	}, sourceInfo.IsDir(), nil
}

func resolveConfigSource(configPath, source string) (string, error) {
	expanded, err := fs.ExpandPath(source)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(expanded) {
		return expanded, nil
	}
	return filepath.Abs(filepath.Join(configPath, expanded))
}

func sortedConfigPaths(configs map[string]*types.Config) []string {
	paths := make([]string, 0, len(configs))
	for path := range configs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func sortEntries(entries []types.FileEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Target != entries[j].Target {
			return entries[i].Target < entries[j].Target
		}
		if entries[i].Action != entries[j].Action {
			return entries[i].Action < entries[j].Action
		}
		return entries[i].Source < entries[j].Source
	})
}

func pathMatchesOrContains(path, prefix string) bool {
	cleanPath := filepath.Clean(path)
	cleanPrefix := filepath.Clean(prefix)
	return cleanPath == cleanPrefix || strings.HasPrefix(cleanPath, cleanPrefix+string(filepath.Separator))
}

func trimPathPrefix(path, prefix string) (string, bool) {
	if prefix == "" {
		return "", false
	}
	cleanPath := filepath.Clean(path)
	cleanPrefix := filepath.Clean(prefix)
	if cleanPath == cleanPrefix {
		return "", true
	}
	if strings.HasPrefix(cleanPath, cleanPrefix+string(filepath.Separator)) {
		return strings.TrimPrefix(cleanPath, cleanPrefix+string(filepath.Separator)), true
	}
	return "", false
}

func remappedTarget(target, suffix string) string {
	if suffix == "" {
		return target
	}
	return filepath.Join(target, strings.TrimPrefix(suffix, string(filepath.Separator)))
}

// Package config handles .cdm.conf.json configuration file parsing
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodgear/cdm/pkg/types"
)

const ConfigFileName = ".cdm.conf.json"

// Loader handles configuration file loading
type Loader struct{}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{}
}

// Load loads configuration from a source directory
func (l *Loader) Load(sourcePath string) (*types.Config, error) {
	configPath := filepath.Join(sourcePath, ConfigFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, return empty config
			return &types.Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config types.Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("failed to parse config file %s: trailing data after JSON object", configPath)
		}
		return nil, fmt.Errorf("failed to parse config file %s: trailing data after JSON object: %w", configPath, err)
	}

	return &config, nil
}

// LoadAll loads configurations from multiple source directories
// Includes recursive loading of subdirectory configs
func (l *Loader) LoadAll(sourcePaths []string) (map[string]*types.Config, error) {
	configs := make(map[string]*types.Config)

	if err := l.MigrateAll(sourcePaths); err != nil {
		return nil, err
	}

	for _, path := range sourcePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
		}

		// Load root config
		config, err := l.Load(absPath)
		if err != nil {
			return nil, err
		}
		configs[absPath] = config

		// Recursively load subdirectory configs
		subConfigs, err := l.loadRecursive(absPath, absPath)
		if err != nil {
			return nil, err
		}
		for subPath, subConfig := range subConfigs {
			configs[subPath] = subConfig
		}
	}

	return configs, nil
}

// loadRecursive recursively finds and loads all .cdm.conf.json files
func (l *Loader) loadRecursive(basePath, currentPath string) (map[string]*types.Config, error) {
	configs := make(map[string]*types.Config)

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subDirPath := filepath.Join(currentPath, entry.Name())

		// Try to load config from this subdirectory
		config, err := l.Load(subDirPath)
		if err != nil {
			return nil, err
		}

		// Only add if config has content (not empty)
		if config.Version != "" || len(config.Remaps) > 0 ||
			len(config.ExternalLinks) > 0 || len(config.CopyIfNotExist) > 0 ||
			len(config.Copy) > 0 || len(config.Exclude) > 0 || len(config.LinkFolders) > 0 ||
			len(config.Repos) > 0 || config.Hooks != nil {
			configs[subDirPath] = config
		}

		// Recurse into subdirectories
		subConfigs, err := l.loadRecursive(basePath, subDirPath)
		if err != nil {
			return nil, err
		}
		for subPath, subConfig := range subConfigs {
			configs[subPath] = subConfig
		}
	}

	return configs, nil
}

// MigrateAll rewrites legacy CDM meta config files before strict loading.
func (l *Loader) MigrateAll(sourcePaths []string) error {
	for _, sourcePath := range sourcePaths {
		absPath, err := filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %w", sourcePath, err)
		}

		if err := filepath.WalkDir(absPath, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Name() != ConfigFileName {
				return nil
			}
			if err := migrateConfigFile(path); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to migrate configs under %s: %w", absPath, err)
		}
	}
	return nil
}

type rawConfig struct {
	Version          string              `json:"version,omitempty"`
	Remaps           []types.PathMapping `json:"remaps,omitempty"`
	ExternalLinks    []types.PathMapping `json:"externalLinks,omitempty"`
	CopyIfNotExist   []types.PathMapping `json:"copyIfNotExist,omitempty"`
	Copy             []types.PathMapping `json:"copy,omitempty"`
	Exclude          []string            `json:"exclude,omitempty"`
	LinkFolders      []string            `json:"linkFolders,omitempty"`
	Hooks            *types.Hooks        `json:"hooks,omitempty"`
	Repos            []types.RepoConfig  `json:"repos,omitempty"`
	PathMappings     []types.PathMapping `json:"pathMappings,omitempty"`
	FileMappings     []types.PathMapping `json:"fileMappings,omitempty"`
	ConfigIfNotExist []types.PathMapping `json:"configIfNotExist,omitempty"`
}

func migrateConfigFile(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	knownKeys := map[string]bool{
		"version":          true,
		"remaps":           true,
		"externalLinks":    true,
		"copyIfNotExist":   true,
		"copy":             true,
		"exclude":          true,
		"linkFolders":      true,
		"hooks":            true,
		"repos":            true,
		"pathMappings":     true,
		"fileMappings":     true,
		"configIfNotExist": true,
	}
	for key := range keys {
		if !knownKeys[key] {
			return fmt.Errorf("unknown config field %q in %s", key, configPath)
		}
	}

	var old rawConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return fmt.Errorf("failed to decode config file %s: %w", configPath, err)
	}

	needsMigration := hasLegacyFields(keys)
	if !needsMigration {
		return nil
	}

	next := types.Config{
		Version:        old.Version,
		Remaps:         append([]types.PathMapping{}, old.Remaps...),
		ExternalLinks:  append([]types.PathMapping{}, old.ExternalLinks...),
		CopyIfNotExist: append([]types.PathMapping{}, old.CopyIfNotExist...),
		Copy:           append([]types.PathMapping{}, old.Copy...),
		Exclude:        append([]string{}, old.Exclude...),
		LinkFolders:    append([]string{}, old.LinkFolders...),
		Hooks:          old.Hooks,
		Repos:          append([]types.RepoConfig{}, old.Repos...),
	}

	next.CopyIfNotExist = append(next.CopyIfNotExist, old.ConfigIfNotExist...)
	next.CopyIfNotExist = append(next.CopyIfNotExist, old.FileMappings...)
	for _, mapping := range old.PathMappings {
		if isExternalMapping(mapping.Source) {
			next.ExternalLinks = append(next.ExternalLinks, mapping)
		} else {
			next.Remaps = append(next.Remaps, mapping)
		}
	}

	migrated, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal migrated config %s: %w", configPath, err)
	}
	migrated = append(migrated, '\n')

	info, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("failed to stat config file %s: %w", configPath, err)
	}

	backupPath := fmt.Sprintf("%s.backup.%s", configPath, time.Now().Format("20060102_150405.000000000"))
	if err := os.WriteFile(backupPath, data, info.Mode()); err != nil {
		return fmt.Errorf("failed to backup config %s to %s: %w", configPath, backupPath, err)
	}

	if err := os.WriteFile(configPath, migrated, info.Mode()); err != nil {
		return fmt.Errorf("failed to write migrated config %s: %w", configPath, err)
	}

	fmt.Printf("[MIGRATE] %s -> %s\n", configPath, backupPath)
	return nil
}

func isExternalMapping(source string) bool {
	return strings.HasPrefix(source, "~") || filepath.IsAbs(source)
}

func hasLegacyFields(keys map[string]json.RawMessage) bool {
	for _, key := range []string{"pathMappings", "fileMappings", "configIfNotExist"} {
		if _, ok := keys[key]; ok {
			return true
		}
	}
	return false
}

// Package config handles .cdm.conf.json configuration file parsing
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	return &config, nil
}

// LoadAll loads configurations from multiple source directories
// Includes recursive loading of subdirectory configs
func (l *Loader) LoadAll(sourcePaths []string) (map[string]*types.Config, error) {
	configs := make(map[string]*types.Config)

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
		if config.Version != "" || len(config.PathMappings) > 0 || 
			len(config.Exclude) > 0 || len(config.LinkFolders) > 0 || config.Hooks != nil {
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

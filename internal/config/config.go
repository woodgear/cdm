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
func (l *Loader) LoadAll(sourcePaths []string) (map[string]*types.Config, error) {
	configs := make(map[string]*types.Config)

	for _, path := range sourcePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
		}

		config, err := l.Load(absPath)
		if err != nil {
			return nil, err
		}

		configs[absPath] = config
	}

	return configs, nil
}

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllMigratesEmptyLegacyFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ConfigFileName)
	input := []byte(`{
  "version": "1.0.0",
  "pathMappings": [],
  "fileMappings": [],
  "configIfNotExist": []
}`)
	if err := os.WriteFile(configPath, input, 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := NewLoader().LoadAll([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if configs[dir] == nil {
		t.Fatalf("expected config for %s", dir)
	}

	migrated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacyField := range [][]byte{[]byte("pathMappings"), []byte("fileMappings"), []byte("configIfNotExist")} {
		if bytes.Contains(migrated, legacyField) {
			t.Fatalf("expected migrated config to omit legacy field %q:\n%s", legacyField, migrated)
		}
	}

	backups, err := filepath.Glob(configPath + ".backup.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected one backup file, got %d", len(backups))
	}
}

func TestLoadRejectsTrailingJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`{} {}`), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewLoader().Load(dir); err == nil {
		t.Fatal("expected trailing JSON to fail")
	}
}

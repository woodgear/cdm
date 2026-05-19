package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodgear/cdm/pkg/types"
)

func TestGenerateCopyTaskExcludesDefaultLink(t *testing.T) {
	sourceRoot := t.TempDir()
	configSource := filepath.Join(sourceRoot, "home", ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configSource), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configSource, []byte("model = \"test\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	config := []byte(`{
  "copyIfNotExist": [
    {
      "source": "home/.codex/config.toml",
      "target": "~/.codex/config.toml"
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(sourceRoot, ".cdm.conf.json"), config, 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := NewGenerator(false).Generate([]string{sourceRoot})
	if err != nil {
		t.Fatal(err)
	}

	if plan.Stats.Total != 1 || plan.Stats.CopyIfNotExist != 1 || plan.Stats.Link != 0 {
		t.Fatalf("unexpected stats: %+v", plan.Stats)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(plan.Tasks))
	}
	task := plan.Tasks[0]
	if task.Action != types.ActionCopyIfNotExist {
		t.Fatalf("expected copy-if-not-exist task, got %s", task.Action)
	}
	if task.Source != configSource {
		t.Fatalf("unexpected task source: %s", task.Source)
	}
}

func TestPathMatchesOrContainsUsesPathBoundary(t *testing.T) {
	if !pathMatchesOrContains(".config/foo/bar", ".config/foo") {
		t.Fatal("expected child path to match")
	}
	if pathMatchesOrContains(".config/foobar", ".config/foo") {
		t.Fatal("expected sibling prefix to not match")
	}
}

package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainSourceAvoidsForbiddenProviderAndDynamicTypes(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read domain dir: %v", err)
	}

	forbidden := []string{
		"github.com/open" + "ai",
		"github.com/anth" + "ropic",
		"json.RawMessage",
		"map[string]any",
		"map[string]interface{}",
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}

		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		text := string(content)
		for _, token := range forbidden {
			if strings.Contains(text, token) {
				t.Fatalf("forbidden token %q found in %s", token, name)
			}
		}
	}
}

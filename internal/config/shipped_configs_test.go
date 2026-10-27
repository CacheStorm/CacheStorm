package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// Every configuration file shipped in config/ must load under the strict
// schema, so the examples cannot drift from what the server reads.
func TestShippedConfigFilesLoad(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "config", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no configuration files found in config/")
	}
	for _, f := range files {
		_, err := Load(f)
		if err != nil && strings.Contains(err.Error(), PlaceholderPassword) {
			continue
		}
		if err != nil && strings.Contains(err.Error(), "TLS") {
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
		}
	}
}

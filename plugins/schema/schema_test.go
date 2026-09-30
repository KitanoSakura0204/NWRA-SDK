package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// testdata/manifests.json is shared with the host: Backend
// internal/modules/pluginmarket/testdata/manifests.json is a byte-identical copy
// that the host's validator must judge the same way, so the schema and the Go validator
// cannot drift apart unnoticed.
type vector struct {
	Name     string          `json:"name"`
	Valid    bool            `json:"valid"`
	Manifest json.RawMessage `json:"manifest"`
}

func loadVectors(t *testing.T) []vector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "manifests.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list []vector
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestManifestVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		errs := ValidatePlugin(v.Manifest)
		if v.Valid && len(errs) > 0 {
			t.Errorf("%s: want valid, got %v", v.Name, errs)
		}
		if !v.Valid && len(errs) == 0 {
			t.Errorf("%s: want invalid, the schema accepted it", v.Name)
		}
	}
}

// The examples of this SDK must satisfy the schema.
func TestExamples(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "examples", "*", "plugin.json"))
	if len(files) == 0 {
		t.Fatal("no example manifests found")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if errs := ValidatePlugin(raw); len(errs) > 0 {
			t.Errorf("%s: %v", f, errs)
		}
	}
}

func TestIndexExample(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if errs := ValidateIndex(raw); len(errs) > 0 {
		t.Fatalf("index example: %v", errs)
	}
	// A 1.0 index (the format before this standard) stays valid.
	old := `{"name":"Old","plugins":[{"id":"com.example.hello","name":"Hello","versions":[{"version":"1.0.0","url":"hello.zip","sha256":"` +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" + `"}]}]}`
	if errs := ValidateIndex([]byte(old)); len(errs) > 0 {
		t.Fatalf("1.0 index: %v", errs)
	}
	bad := `{"name":"Bad","plugins":[{"id":"x","name":"X","versions":[]}]}`
	if errs := ValidateIndex([]byte(bad)); len(errs) < 2 {
		t.Fatalf("bad index: want at least the id and the empty versions reported, got %v", errs)
	}
}

func TestCategoriesMatchSchema(t *testing.T) {
	var doc struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(PluginSchema, &doc); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(doc.Defs["category"].Enum, Categories) {
		t.Fatalf("schema categories %v != Categories %v", doc.Defs["category"].Enum, Categories)
	}
}

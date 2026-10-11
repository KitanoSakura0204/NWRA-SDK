package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSemverPrereleaseIdentifierOrdering(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"1.0.0-beta.10", "1.0.0-beta.2", 1},
		{"1.0.0-beta.2", "1.0.0-beta.10", -1},
		{"1.0.0-beta.99999999999999999999", "1.0.0-beta.9999999999999999999", 1},
		{"1.0.0-beta.2", "1.0.0-beta.a", -1},
		{"1.0.0-beta.z", "1.0.0-beta.20", 1},
		{"1.0.0-beta", "1.0.0-beta.1", -1},
		{"1.0.0-beta.1", "1.0.0-beta", 1},
		{"1.0.0-beta.2", "1.0.0-beta.2", 0},
		{"1.0.0-beta.99", "1.0.0", -1},
		{"1.0.0", "1.0.0-beta.99", 1},
		{"1.10.0-beta", "1.9.0", 1},
	} {
		if got := compareSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestIndexLatestUsesNumericPrerelease(t *testing.T) {
	dir := t.TempDir()
	byID := map[string]*indexPlugin{}
	for _, version := range []string{"1.0.0-beta.10", "1.0.0-beta.2"} {
		data, err := writeZip(demoFiles(version), nil)
		if err != nil {
			t.Fatal(err)
		}
		name := "com.example.demo-" + version + ".zip"
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := addPackage(byID, file, dir, "", name, nil, true); err != nil {
			t.Fatal(err)
		}
	}
	plugin := byID["com.example.demo"]
	if plugin == nil || plugin.latest != "1.0.0-beta.10" || plugin.Icon != "assets/com.example.demo/1.0.0-beta.10/icon.png" {
		t.Fatalf("latest prerelease listing regressed: %+v", plugin)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\n0000")

const listingManifest = `{"id":"com.example.demo","name":"Demo","version":"%s","author":"Me","kind":"declarative",
 "capabilities":["ui"],"icon":"icon.png","categories":["account"],"changelog":{"en":"v%s"},
 "engines":{"nowork":">=1.1.0 <2.0.0","abi":1},
 "ui":[{"slot":"home.page","title":{"en":"Demo"},"entry":"ui/index.html"}]}`

func demoFiles(version string) map[string][]byte {
	return map[string][]byte{
		manifestFile:    []byte(strings.ReplaceAll(listingManifest, "%s", version)),
		"icon.png":      pngBytes,
		"ui/index.html": []byte("<p>hi</p>"),
	}
}

func TestLintFiles(t *testing.T) {
	if p := lintFiles(demoFiles("1.1.0")); len(p) != 0 {
		t.Fatalf("clean package reported %v", p)
	}
	files := demoFiles("1.1.0")
	files["icon.png"] = []byte("GIF89a")
	delete(files, "ui/index.html")
	p := strings.Join(lintFiles(files), "\n")
	for _, want := range []string{"icon icon.png: the content does not match", "UI entry ui/index.html is missing"} {
		if !strings.Contains(p, want) {
			t.Errorf("want %q in\n%s", want, p)
		}
	}
	files = demoFiles("1.1")
	if p := lintFiles(files); len(p) == 0 {
		t.Fatal("an invalid version passed the schema")
	}
}

func TestIndexPackages(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"1.0.0", "1.1.0", "1.2.0"} {
		data, err := writeZip(demoFiles(v), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "com.example.demo-"+v+".zip"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	byID := map[string]*indexPlugin{}
	yanked := map[string]string{"com.example.demo@1.2.0": "breaks sign-in"}
	for _, v := range []string{"1.0.0", "1.2.0", "1.1.0"} {
		name := "com.example.demo-" + v + ".zip"
		if err := addPackage(byID, filepath.Join(dir, name), dir, "", name, yanked, true); err != nil {
			t.Fatal(err)
		}
	}
	p := byID["com.example.demo"]
	if p == nil || len(p.Versions) != 3 {
		t.Fatalf("plugin entry: %+v", p)
	}
	// The listing follows the highest version that is not yanked.
	if p.latest != "1.1.0" || p.Icon != "assets/com.example.demo/1.1.0/icon.png" {
		t.Fatalf("latest %s icon %s", p.latest, p.Icon)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "com.example.demo", "1.1.0", "icon.png")); err != nil {
		t.Fatalf("icon not copied: %v", err)
	}
	for _, v := range p.Versions {
		if v.Yanked != (v.Version == "1.2.0") || v.Engines == nil || v.Changelog["en"] != "v"+v.Version {
			t.Errorf("version %+v", v)
		}
	}
	if compareSemver("1.0.0-beta.1", "1.0.0") >= 0 || compareSemver("1.10.0", "1.9.9") <= 0 {
		t.Fatal("compareSemver order")
	}
}

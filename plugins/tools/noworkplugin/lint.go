package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/wangework/nowork-sdk-plugins/schema"
)

// Package limits of the host (Backend internal/modules/plugins/package.go) and the listing
// limits of the standard 1.1 (internal/modules/pluginmarket/pluginstd).
const (
	maxPackageBytes  = 10 << 20
	maxUnpackedBytes = 32 << 20
	maxFileBytes     = 16 << 20
	maxEntries       = 500
	maxIconBytes     = 256 << 10
	maxShotBytes     = 1 << 20
)

// lint checks a plugin directory, a package or an index against the standard 1.1: the JSON
// Schema first, then what a schema cannot say (referenced files, image types and sizes,
// package limits, capability pairing). Problems are printed one per line; the exit code is 1
// when there is any.
func lint(args []string) error {
	set := flag.NewFlagSet("lint", flag.ExitOnError)
	dir := set.String("dir", "", "plugin directory (as pack would collect it)")
	zipPath := set.String("zip", "", "plugin package")
	indexPath := set.String("index", "", "market index.json")
	_ = set.Parse(args)
	var problems []string
	switch {
	case *indexPath != "":
		raw, err := os.ReadFile(*indexPath)
		if err != nil {
			return err
		}
		problems = schema.ValidateIndex(raw)
	case *zipPath != "":
		files, _, size, err := readZip(*zipPath)
		if err != nil {
			return err
		}
		if size > maxPackageBytes {
			problems = append(problems, fmt.Sprintf("package: %d bytes, the host accepts at most %d", size, maxPackageBytes))
		}
		problems = append(problems, lintFiles(files)...)
	case *dir != "":
		files, err := collect(*dir)
		if err != nil {
			return err
		}
		problems = lintFiles(files)
	default:
		return errors.New("lint needs -dir, -zip or -index")
	}
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problem(s)", len(problems))
	}
	fmt.Println("OK: standard", schema.Standard)
	return nil
}

// lintManifest is the part of plugin.json the package checks need.
type lintManifest struct {
	Kind         string   `json:"kind"`
	Capabilities []string `json:"capabilities"`
	Icon         string   `json:"icon"`
	Screenshots  []struct {
		Path string `json:"path"`
	} `json:"screenshots"`
	UI []struct {
		Entry string `json:"entry"`
	} `json:"ui"`
	Wasm *struct {
		Module string `json:"module"`
	} `json:"wasm"`
	HTTP *struct {
		AllowHosts []string `json:"allowHosts"`
	} `json:"http"`
}

func lintFiles(files map[string][]byte) []string {
	raw := files[manifestFile]
	if raw == nil {
		return []string{"package: no plugin.json at the root"}
	}
	problems := schema.ValidatePlugin(raw)
	if len(files) > maxEntries {
		problems = append(problems, fmt.Sprintf("package: %d files, at most %d", len(files), maxEntries))
	}
	total := 0
	for n, b := range files {
		total += len(b)
		if len(b) > maxFileBytes {
			problems = append(problems, fmt.Sprintf("package: %s is larger than %d MB", n, maxFileBytes>>20))
		}
	}
	if total > maxUnpackedBytes {
		problems = append(problems, fmt.Sprintf("package: %d bytes unpacked, at most %d", total, maxUnpackedBytes))
	}
	var m lintManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return append(problems, "plugin.json: "+err.Error())
	}
	need := func(p, what string) []byte {
		b, ok := files[p]
		if p != "" && !ok {
			problems = append(problems, fmt.Sprintf("package: %s %s is missing", what, p))
		}
		return b
	}
	if m.Kind == "wasm" {
		mod := "plugin.wasm"
		if m.Wasm != nil && m.Wasm.Module != "" {
			mod = m.Wasm.Module
		}
		need(mod, "wasm module")
	}
	for _, u := range m.UI {
		need(u.Entry, "UI entry")
	}
	if b := need(m.Icon, "icon"); b != nil {
		problems = append(problems, checkImage("icon "+m.Icon, b, maxIconBytes, false)...)
	}
	for _, s := range m.Screenshots {
		if b := need(s.Path, "screenshot"); b != nil {
			problems = append(problems, checkImage("screenshot "+s.Path, b, maxShotBytes, true)...)
		}
	}
	hasHTTP := false
	for _, c := range m.Capabilities {
		hasHTTP = hasHTTP || c == "http"
	}
	if hasHTTP != (m.HTTP != nil && len(m.HTTP.AllowHosts) > 0) {
		problems = append(problems, "plugin.json: the capability http and http.allowHosts go together")
	}
	return problems
}

// checkImage checks the real content type (magic bytes, not the extension) and the size.
func checkImage(label string, b []byte, limit int, jpegOK bool) []string {
	var out []string
	if len(b) > limit {
		out = append(out, fmt.Sprintf("%s: %d bytes, at most %d", label, len(b), limit))
	}
	ext := strings.ToLower(path.Ext(label))
	png := bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n"))
	webp := len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP"
	jpeg := bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff})
	ok := (ext == ".png" && png) || (ext == ".webp" && webp) || (jpegOK && (ext == ".jpg" || ext == ".jpeg") && jpeg)
	if !ok {
		out = append(out, label+": the content does not match the file type (png / webp"+map[bool]string{true: " / jpeg", false: ""}[jpegOK]+")")
	}
	return out
}

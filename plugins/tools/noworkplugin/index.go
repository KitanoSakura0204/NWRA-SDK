package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The index is standard 1.1 (SDK/plugins/schema/index.schema.json); hosts that only read
// 1.0 ignore the new fields.
type indexVersion struct {
	Version      string            `json:"version"`
	URL          string            `json:"url"`
	SHA256       string            `json:"sha256"`
	KeyID        string            `json:"keyId,omitempty"`
	Size         int               `json:"size"`
	PublishedAt  string            `json:"publishedAt,omitempty"`
	Changelog    map[string]string `json:"changelog,omitempty"`
	Permissions  []string          `json:"permissions"`
	Capabilities []string          `json:"capabilities"`
	Engines      *engines          `json:"engines,omitempty"`
	Yanked       bool              `json:"yanked,omitempty"`
	YankedReason string            `json:"yankedReason,omitempty"`
}

type engines struct {
	Lounge string `json:"lounge,omitempty"`
	ABI    int    `json:"abi,omitempty"`
}

type screenshotRef struct {
	Path    string            `json:"path,omitempty"`
	URL     string            `json:"url,omitempty"`
	Caption map[string]string `json:"caption,omitempty"`
}

type indexPlugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Author      string            `json:"author"`
	Description string            `json:"description"`
	Kind        string            `json:"kind"`
	Scope       string            `json:"scope,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	Support     map[string]string `json:"support,omitempty"`
	Locales     map[string]any    `json:"locales,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Screenshots []screenshotRef   `json:"screenshots,omitempty"`
	Categories  []string          `json:"categories,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
	Versions    []indexVersion    `json:"versions"`
	latest      string
}

// manifestListing is what the index reads from plugin.json (identity and listing fields).
type manifestListing struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Author       string            `json:"author"`
	Description  string            `json:"description"`
	Kind         string            `json:"kind"`
	Scope        string            `json:"scope"`
	Homepage     string            `json:"homepage"`
	Repository   string            `json:"repository"`
	Support      map[string]string `json:"support"`
	Locales      map[string]any    `json:"locales"`
	Icon         string            `json:"icon"`
	Screenshots  []screenshotRef   `json:"screenshots"`
	Categories   []string          `json:"categories"`
	Keywords     []string          `json:"keywords"`
	Permissions  []string          `json:"permissions"`
	Capabilities []string          `json:"capabilities"`
	Engines      *engines          `json:"engines"`
	Changelog    map[string]string `json:"changelog"`
}

type publisher struct {
	KeyID     string `json:"keyId"`
	Name      string `json:"name"`
	URL       string `json:"url,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
}

type revokedKey struct {
	KeyID     string `json:"keyId"`
	Reason    string `json:"reason,omitempty"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

// index writes a standard 1.1 market index for the packages in -dir. The icon and the
// screenshots of each version are copied to <dir>/assets/<id>/<version>/ so a static host
// serves them next to the packages; the plugin entry takes its listing from the highest
// version that is not yanked.
func index(args []string) error {
	set := flag.NewFlagSet("index", flag.ExitOnError)
	dir := set.String("dir", "dist", "directory with the packages")
	name := set.String("name", "Plugins", "market name")
	base := set.String("base", "", "URL prefix of the packages and assets (default: relative to the index)")
	siteURL := set.String("url", "", "the market's own https site (optional)")
	pubPath := set.String("publisher-key", "", "publisher public key (.pub from keygen): writes the publisher block")
	pubName := set.String("publisher-name", "", "publisher name (default: the market name)")
	pubURL := set.String("publisher-url", "", "publisher https URL (optional)")
	revokedPath := set.String("revoked", "", `JSON file: [{"keyId":"…","reason":"…","revokedAt":"RFC 3339"}]`)
	yankedPath := set.String("yanked", "", `JSON file: {"<id>@<version>": "reason"}`)
	assets := set.Bool("assets", true, "copy icons and screenshots to <dir>/assets")
	_ = set.Parse(args)

	yanked := map[string]string{}
	if *yankedPath != "" {
		if err := readJSONFile(*yankedPath, &yanked); err != nil {
			return err
		}
	}
	out := struct {
		Format      string         `json:"format"`
		Name        string         `json:"name"`
		URL         string         `json:"url,omitempty"`
		UpdatedAt   string         `json:"updatedAt"`
		Publisher   *publisher     `json:"publisher,omitempty"`
		RevokedKeys []revokedKey   `json:"revokedKeys,omitempty"`
		Plugins     []*indexPlugin `json:"plugins"`
	}{Format: "1.1", Name: *name, URL: *siteURL, UpdatedAt: time.Now().UTC().Format(time.RFC3339), Plugins: []*indexPlugin{}}
	if *pubPath != "" {
		pub, err := readPublic(*pubPath)
		if err != nil {
			return err
		}
		out.Publisher = &publisher{KeyID: keyID(pub), Name: *pubName, URL: *pubURL, PublicKey: base64.StdEncoding.EncodeToString(pub)}
		if out.Publisher.Name == "" {
			out.Publisher.Name = *name
		}
	}
	if *revokedPath != "" {
		if err := readJSONFile(*revokedPath, &out.RevokedKeys); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(*dir)
	if err != nil {
		return err
	}
	byID := map[string]*indexPlugin{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		if err := addPackage(byID, filepath.Join(*dir, e.Name()), *dir, *base, e.Name(), yanked, *assets); err != nil {
			return err
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := byID[id]
		sort.Slice(p.Versions, func(i, j int) bool { return compareSemver(p.Versions[i].Version, p.Versions[j].Version) > 0 })
		out.Plugins = append(out.Plugins, p)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func addPackage(byID map[string]*indexPlugin, file, dir, base, zipName string, yanked map[string]string, copyAssets bool) error {
	files, sigRaw, size, err := readZip(file)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	var m manifestListing
	if err := json.Unmarshal(files[manifestFile], &m); err != nil {
		return fmt.Errorf("%s: bad plugin.json: %w", file, err)
	}
	m.ID, m.Version = strings.TrimSpace(m.ID), strings.TrimSpace(m.Version)
	if m.ID == "" || m.Version == "" {
		return fmt.Errorf("%s: plugin.json needs id and version", file)
	}
	var sig signature
	_ = json.Unmarshal(sigRaw, &sig)
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	info, _ := os.Stat(file)
	v := indexVersion{Version: m.Version, URL: base + zipName, SHA256: hex.EncodeToString(sum[:]), KeyID: sig.KeyID, Size: size,
		Changelog: m.Changelog, Permissions: nonNil(m.Permissions), Capabilities: nonNil(m.Capabilities), Engines: m.Engines}
	if info != nil {
		v.PublishedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	if reason, ok := yanked[m.ID+"@"+m.Version]; ok {
		v.Yanked, v.YankedReason = true, reason
	}
	cur, ok := byID[m.ID]
	if !ok {
		cur = &indexPlugin{ID: m.ID}
		byID[m.ID] = cur
	}
	cur.Versions = append(cur.Versions, v)
	if v.Yanked || (cur.latest != "" && compareSemver(m.Version, cur.latest) < 0) {
		if cur.Name == "" { // only yanked versions so far: keep a listing anyway
			cur.Name, cur.Author, cur.Description, cur.Kind = m.Name, m.Author, m.Description, m.Kind
		}
		return nil
	}
	cur.latest = m.Version
	cur.Name, cur.Author, cur.Description, cur.Kind, cur.Scope = m.Name, m.Author, m.Description, m.Kind, m.Scope
	cur.Homepage, cur.Repository, cur.Support, cur.Locales = m.Homepage, m.Repository, m.Support, m.Locales
	cur.Categories, cur.Keywords, cur.UpdatedAt = m.Categories, m.Keywords, v.PublishedAt
	cur.Icon, cur.Screenshots = "", nil
	prefix := path.Join("assets", m.ID, m.Version)
	publish := func(p string) (string, error) {
		body, ok := files[p]
		if !ok {
			return "", fmt.Errorf("%s: %s is not in the package", file, p)
		}
		if copyAssets {
			target := filepath.Join(dir, filepath.FromSlash(path.Join(prefix, p)))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(target, body, 0o644); err != nil {
				return "", err
			}
		}
		return base + path.Join(prefix, p), nil
	}
	if m.Icon != "" {
		if cur.Icon, err = publish(m.Icon); err != nil {
			return err
		}
	}
	for _, s := range m.Screenshots {
		u, err := publish(s.Path)
		if err != nil {
			return err
		}
		cur.Screenshots = append(cur.Screenshots, screenshotRef{URL: u, Caption: s.Caption})
	}
	return nil
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func readJSONFile(file string, v any) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	return nil
}

func readPublic(file string) ([]byte, error) {
	text, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(text)))
	if err != nil || len(pub) != 32 {
		return nil, errors.New(file + ": not a base64 ed25519 public key")
	}
	return pub, nil
}

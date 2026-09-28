// Command loungeplugin creates signing keys, packs and signs plugin packages, verifies them
// and writes a market index. Standard library only.
//
//	loungeplugin keygen -out dev             → dev.key (PKCS#8 PEM, keep it secret) + dev.pub (base64)
//	loungeplugin pack -dir ./hello -key dev.key -out dist
//	loungeplugin verify -zip dist/com.example.hello-1.0.0.zip -pub dev.pub
//	loungeplugin index -dir dist -name "My plugins" > index.json
//
// The package is a zip of plugin.json plus every other file of the directory (hidden files,
// *.go, go.mod / go.sum, build scripts and dist/ are left out) and signature.json. The
// signature is ed25519 over "lounge-plugin-v1\n<id>\n<version>\n<digest>\n", where digest is
// the hex SHA-256 over the sorted lines "<path>\x00<hex sha256 of the file>\n" of every file
// except signature.json. The user centre verifies exactly that (Backend
// internal/modules/plugins/sign.go).
package main

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	manifestFile  = "plugin.json"
	signatureFile = "signature.json"
	sigDomain     = "lounge-plugin-v1"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "pack":
		err = pack(os.Args[2:])
	case "verify":
		err = verify(os.Args[2:])
	case "index":
		err = index(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loungeplugin:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: loungeplugin keygen|pack|verify|index [flags] (see -h of each)")
	os.Exit(2)
}

// ---- the signature scheme (must match the backend byte for byte) ----

func contentDigest(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		sum := sha256.Sum256(files[n])
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write([]byte(hex.EncodeToString(sum[:])))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func signedMessage(id, version, digest string) []byte {
	return []byte(sigDomain + "\n" + id + "\n" + version + "\n" + digest + "\n")
}

func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

type signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId"`
	Signature string `json:"signature"`
}

type manifestHead struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

func readHead(raw []byte) (manifestHead, error) {
	var m manifestHead
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("plugin.json: %w", err)
	}
	m.ID, m.Version = strings.TrimSpace(m.ID), strings.TrimSpace(m.Version)
	if m.ID == "" || m.Version == "" {
		return m, errors.New("plugin.json needs id and version")
	}
	return m, nil
}

// ---- commands ----

func keygen(args []string) error {
	set := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := set.String("out", "publisher", "file prefix: <out>.key (private, PKCS#8 PEM) and <out>.pub (base64)")
	_ = set.Parse(args)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out+".key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return err
	}
	b64 := base64.StdEncoding.EncodeToString(pub)
	if err := os.WriteFile(*out+".pub", []byte(b64+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s.key (keep it secret) and %s.pub\n  key id     %s\n  public key %s\n", *out, *out, keyID(pub), b64)
	return nil
}

func loadPrivate(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New(path + ": not a PEM file")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New(path + ": not an ed25519 key")
	}
	return priv, nil
}

// collect reads the files that go into the package (sources, build files, keys and hidden
// files are left out).
func collect(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || (d.IsDir() && name == "dist") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		switch {
		case name == "go.mod", name == "go.sum", name == "build.sh", name == "build.ps1", name == signatureFile,
			strings.HasSuffix(name, ".go"), strings.HasSuffix(name, ".key"), strings.HasSuffix(name, ".pub"):
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[rel] = data
		return nil
	})
	if err == nil && files[manifestFile] == nil {
		err = errors.New(dir + ": no plugin.json")
	}
	return files, err
}

var fixedTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC) // reproducible zips

func writeZip(files map[string][]byte, sig []byte) ([]byte, error) {
	all := map[string][]byte{}
	for k, v := range files {
		all[k] = v
	}
	if sig != nil {
		all[signatureFile] = sig
	}
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Deflate, Modified: fixedTime})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(all[n]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func pack(args []string) error {
	set := flag.NewFlagSet("pack", flag.ExitOnError)
	dir := set.String("dir", ".", "plugin directory (plugin.json at its root)")
	key := set.String("key", "", "private key from keygen; empty = unsigned package")
	out := set.String("out", "dist", "output directory")
	_ = set.Parse(args)
	files, err := collect(*dir)
	if err != nil {
		return err
	}
	head, err := readHead(files[manifestFile])
	if err != nil {
		return err
	}
	var sig []byte
	kid := ""
	if *key != "" {
		priv, err := loadPrivate(*key)
		if err != nil {
			return err
		}
		kid = keyID(priv.Public().(ed25519.PublicKey))
		raw := ed25519.Sign(priv, signedMessage(head.ID, head.Version, contentDigest(files)))
		sig, _ = json.Marshal(signature{Algorithm: "ed25519", KeyID: kid, Signature: base64.StdEncoding.EncodeToString(raw)})
	}
	data, err := writeZip(files, sig)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	name := filepath.Join(*out, head.ID+"-"+head.Version+".zip")
	if err := os.WriteFile(name, data, 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	fmt.Printf("%s\n  files  %d\n  size   %d bytes\n  sha256 %s\n", name, len(files), len(data), hex.EncodeToString(sum[:]))
	if kid != "" {
		fmt.Printf("  keyId  %s\n", kid)
	} else {
		fmt.Println("  UNSIGNED: installable only after a super administrator allows unsigned plugins")
	}
	if len(data) > 10<<20 {
		fmt.Fprintln(os.Stderr, "warning: larger than the 10 MB upload limit")
	}
	return nil
}

func readZip(path string) (files map[string][]byte, sig []byte, size int, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, 0, err
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, nil, 0, err
	}
	files = map[string][]byte{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, nil, 0, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, nil, 0, err
		}
		if f.Name == signatureFile {
			sig = b
		} else {
			files[f.Name] = b
		}
	}
	return files, sig, len(raw), nil
}

func verify(args []string) error {
	set := flag.NewFlagSet("verify", flag.ExitOnError)
	zipPath := set.String("zip", "", "package to check")
	pubPath := set.String("pub", "", "publisher public key (the .pub file from keygen)")
	_ = set.Parse(args)
	files, sigRaw, _, err := readZip(*zipPath)
	if err != nil {
		return err
	}
	head, err := readHead(files[manifestFile])
	if err != nil {
		return err
	}
	if sigRaw == nil {
		return errors.New("the package is not signed")
	}
	var sig signature
	if err := json.Unmarshal(sigRaw, &sig); err != nil || sig.Algorithm != "ed25519" {
		return errors.New("signature.json is malformed")
	}
	pubText, err := os.ReadFile(*pubPath)
	if err != nil {
		return err
	}
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(pubText)))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New(*pubPath + ": not a base64 ed25519 public key")
	}
	if sig.KeyID != keyID(pub) {
		return fmt.Errorf("signed with key %s, not with %s", sig.KeyID, keyID(pub))
	}
	raw, err := base64.StdEncoding.DecodeString(sig.Signature)
	if err != nil || !ed25519.Verify(pub, signedMessage(head.ID, head.Version, contentDigest(files)), raw) {
		return errors.New("the signature does not verify: the package was modified after signing")
	}
	fmt.Printf("OK %s %s, signed by key %s\n", head.ID, head.Version, sig.KeyID)
	return nil
}

func index(args []string) error {
	set := flag.NewFlagSet("index", flag.ExitOnError)
	dir := set.String("dir", "dist", "directory with the packages")
	name := set.String("name", "Plugins", "market name")
	base := set.String("base", "", "URL prefix of the packages (default: relative to the index)")
	_ = set.Parse(args)
	type version struct {
		Version string `json:"version"`
		URL     string `json:"url"`
		SHA256  string `json:"sha256"`
		KeyID   string `json:"keyId,omitempty"`
		Size    int    `json:"size"`
	}
	type plugin struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Author      string    `json:"author"`
		Description string    `json:"description"`
		Kind        string    `json:"kind"`
		Homepage    string    `json:"homepage,omitempty"`
		Versions    []version `json:"versions"`
	}
	entries, err := os.ReadDir(*dir)
	if err != nil {
		return err
	}
	byID := map[string]*plugin{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		p := filepath.Join(*dir, e.Name())
		files, sigRaw, size, err := readZip(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		var m plugin
		var head manifestHead
		if json.Unmarshal(files[manifestFile], &m) != nil || json.Unmarshal(files[manifestFile], &head) != nil {
			return fmt.Errorf("%s: bad plugin.json", p)
		}
		var sig signature
		_ = json.Unmarshal(sigRaw, &sig)
		raw, _ := os.ReadFile(p)
		sum := sha256.Sum256(raw)
		cur, ok := byID[m.ID]
		if !ok {
			m.Versions = nil
			cur = &m
			byID[m.ID] = cur
		}
		cur.Versions = append(cur.Versions, version{Version: head.Version, URL: *base + e.Name(), SHA256: hex.EncodeToString(sum[:]), KeyID: sig.KeyID, Size: size})
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := struct {
		Name    string    `json:"name"`
		Plugins []*plugin `json:"plugins"`
	}{Name: *name, Plugins: []*plugin{}}
	for _, id := range ids {
		out.Plugins = append(out.Plugins, byID[id])
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

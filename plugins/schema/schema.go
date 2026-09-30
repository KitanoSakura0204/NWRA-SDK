// Package schema carries the JSON Schemas (2020-12) of the plugin standard 1.1, plugin.json
// and the market index.json, and a small validator for the keyword subset they use, so the
// loungeplugin tool and the tests check files without a third-party library.
//
// The schemas mirror the host's validator (Backend internal/modules/plugins/manifest.go and
// internal/modules/pluginmarket/pluginstd); the host checks more than a schema can express
// (extension points against its registry, handler and capability pairing, file references).
package schema

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

// Standard is the plugin standard version these schemas describe.
const Standard = "1.1"

//go:embed plugin.schema.json
var PluginSchema []byte

//go:embed index.schema.json
var IndexSchema []byte

// Categories is the fixed category list (the host's pluginstd.Categories).
var Categories = []string{"account", "security", "game", "server", "community", "notification", "integration", "analytics", "appearance", "developer", "other"}

var (
	pluginDoc, indexDoc map[string]any
	registry            map[string]map[string]any
)

func init() {
	registry = map[string]map[string]any{}
	for _, raw := range [][]byte{PluginSchema, IndexSchema} {
		var d map[string]any
		if err := json.Unmarshal(raw, &d); err != nil {
			panic("schema: embedded schema is not JSON: " + err.Error())
		}
		id, _ := d["$id"].(string)
		registry[id] = d
		if pluginDoc == nil {
			pluginDoc = d
		} else {
			indexDoc = d
		}
	}
}

// ValidatePlugin checks a plugin.json document; it returns every problem found, sorted.
func ValidatePlugin(raw []byte) []string { return validateRaw(pluginDoc, raw) }

// ValidateIndex checks a market index.json document.
func ValidateIndex(raw []byte) []string { return validateRaw(indexDoc, raw) }

func validateRaw(root map[string]any, raw []byte) []string {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return []string{fmt.Sprintf("not JSON: %v", err)}
	}
	v := &validator{}
	v.check(root, root, doc, "$")
	sort.Strings(v.errs)
	return v.errs
}

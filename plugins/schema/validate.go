package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// validator implements the JSON Schema keywords the two schemas use: $ref (local and to the
// other embedded schema), type, enum, required, properties, additionalProperties,
// propertyNames, minProperties, items, minItems, maxItems, uniqueItems, minLength, maxLength
// (in characters), pattern, minimum, maximum. Annotations (title, description, format) are
// ignored, as the specification allows.
type validator struct{ errs []string }

var (
	reMu    sync.Mutex
	reCache = map[string]*regexp.Regexp{}
)

func compiled(p string) *regexp.Regexp {
	reMu.Lock()
	defer reMu.Unlock()
	if re, ok := reCache[p]; ok {
		return re
	}
	re := regexp.MustCompile(p)
	reCache[p] = re
	return re
}

func (v *validator) fail(path, format string, args ...any) {
	v.errs = append(v.errs, path+": "+fmt.Sprintf(format, args...))
}

// resolve follows "#/$defs/x" inside root or "<$id>#/$defs/x" in the other schema.
func resolve(root map[string]any, ref string) (map[string]any, map[string]any) {
	doc := root
	if i := strings.Index(ref, "#"); i > 0 {
		doc = registry[ref[:i]]
		ref = ref[i:]
	}
	if doc == nil || !strings.HasPrefix(ref, "#/") {
		panic("schema: unresolvable $ref " + ref)
	}
	var cur any = doc
	for _, part := range strings.Split(ref[2:], "/") {
		m, _ := cur.(map[string]any)
		cur = m[part]
	}
	s, ok := cur.(map[string]any)
	if !ok {
		panic("schema: unresolvable $ref " + ref)
	}
	return doc, s
}

func (v *validator) check(root, s map[string]any, val any, path string) {
	if ref, ok := s["$ref"].(string); ok {
		doc, target := resolve(root, ref)
		v.check(doc, target, val, path)
	}
	if t, ok := s["type"].(string); ok && !typeMatches(t, val) {
		v.fail(path, "must be %s", t)
		return
	}
	if enum, ok := s["enum"].([]any); ok && !inEnum(enum, val) {
		v.fail(path, "must be one of %s", enumText(enum))
	}
	switch x := val.(type) {
	case string:
		v.checkString(s, x, path)
	case float64:
		if lo, ok := s["minimum"].(float64); ok && x < lo {
			v.fail(path, "must be >= %v", lo)
		}
		if hi, ok := s["maximum"].(float64); ok && x > hi {
			v.fail(path, "must be <= %v", hi)
		}
	case []any:
		v.checkArray(root, s, x, path)
	case map[string]any:
		v.checkObject(root, s, x, path)
	}
}

func (v *validator) checkString(s map[string]any, x, path string) {
	n := float64(utf8.RuneCountInString(x))
	if lo, ok := s["minLength"].(float64); ok && n < lo {
		v.fail(path, "must have at least %v characters", lo)
	}
	if hi, ok := s["maxLength"].(float64); ok && n > hi {
		v.fail(path, "must have at most %v characters", hi)
	}
	if p, ok := s["pattern"].(string); ok && !compiled(p).MatchString(x) {
		v.fail(path, "%q does not match %s", truncate(x, 60), p)
	}
}

func (v *validator) checkArray(root, s map[string]any, x []any, path string) {
	if lo, ok := s["minItems"].(float64); ok && float64(len(x)) < lo {
		v.fail(path, "must have at least %v items", lo)
	}
	if hi, ok := s["maxItems"].(float64); ok && float64(len(x)) > hi {
		v.fail(path, "must have at most %v items", hi)
	}
	if u, _ := s["uniqueItems"].(bool); u {
		seen := map[string]bool{}
		for _, it := range x {
			k, _ := json.Marshal(it)
			if seen[string(k)] {
				v.fail(path, "has the duplicate item %s", truncate(string(k), 60))
			}
			seen[string(k)] = true
		}
	}
	if items, ok := s["items"].(map[string]any); ok {
		for i, it := range x {
			v.check(root, items, it, fmt.Sprintf("%s[%d]", path, i))
		}
	}
}

func (v *validator) checkObject(root, s map[string]any, x map[string]any, path string) {
	if req, ok := s["required"].([]any); ok {
		for _, r := range req {
			if _, has := x[r.(string)]; !has {
				v.fail(path, "missing required property %s", r)
			}
		}
	}
	if lo, ok := s["minProperties"].(float64); ok && float64(len(x)) < lo {
		v.fail(path, "must have at least %v properties", lo)
	}
	props, _ := s["properties"].(map[string]any)
	names, _ := s["propertyNames"].(map[string]any)
	for k, val := range x {
		sub := path + "." + k
		if names != nil {
			v.check(root, names, k, sub+" (name)")
		}
		if ps, ok := props[k].(map[string]any); ok {
			v.check(root, ps, val, sub)
			continue
		}
		switch ap := s["additionalProperties"].(type) {
		case bool:
			if !ap {
				v.fail(path, "unknown property %s", k)
			}
		case map[string]any:
			v.check(root, ap, val, sub)
		}
	}
}

func typeMatches(t string, val any) bool {
	switch t {
	case "object":
		_, ok := val.(map[string]any)
		return ok
	case "array":
		_, ok := val.([]any)
		return ok
	case "string":
		_, ok := val.(string)
		return ok
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "number":
		_, ok := val.(float64)
		return ok
	case "integer":
		f, ok := val.(float64)
		return ok && f == math.Trunc(f)
	}
	return false
}

func inEnum(enum []any, val any) bool {
	for _, e := range enum {
		if e == val {
			return true
		}
	}
	return false
}

func enumText(enum []any) string {
	parts := make([]string, len(enum))
	for i, e := range enum {
		parts[i] = fmt.Sprint(e)
	}
	return strings.Join(parts, " | ")
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

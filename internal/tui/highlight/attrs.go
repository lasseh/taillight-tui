package highlight

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// AttrsInline renders a JSON attrs object as "key=val key=val" like the web
// GUI. Keys are sorted for stable output — map iteration order is random.
// Returns "" for empty, null, or non-object attrs.
func AttrsInline(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		switch val := attrs[k].(type) {
		case string:
			parts = append(parts, k+"="+val)
		default:
			b, err := json.Marshal(val)
			if err != nil {
				parts = append(parts, k+"="+fmt.Sprintf("%v", val))
				continue
			}
			parts = append(parts, k+"="+string(b))
		}
	}
	return strings.Join(parts, " ")
}

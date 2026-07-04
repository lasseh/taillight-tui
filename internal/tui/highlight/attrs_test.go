package highlight

import (
	"encoding/json"
	"testing"
)

func TestAttrsInline(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "empty object", raw: "{}", want: ""},
		{name: "null", raw: "null", want: ""},
		{name: "invalid json", raw: "{not json", want: ""},
		{name: "non-object", raw: `[1,2]`, want: ""},
		{name: "string values sorted by key", raw: `{"b":"2","a":"1"}`, want: "a=1 b=2"},
		{name: "non-string values marshalled", raw: `{"n":42,"ok":true}`, want: "n=42 ok=true"},
		{name: "nested object", raw: `{"m":{"x":1}}`, want: `m={"x":1}`},
		{name: "mixed", raw: `{"svc":"api","count":3}`, want: "count=3 svc=api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AttrsInline(json.RawMessage(tt.raw)); got != tt.want {
				t.Errorf("AttrsInline(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

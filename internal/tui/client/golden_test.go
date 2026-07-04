package client

// Golden-fixture contract tests. The JSON files under testdata/golden/ are
// verbatim copies of the main repo's generated wire fixtures (see
// testdata/golden/README.md for the source of truth and the refresh
// procedure). Each case decodes a fixture into the client type that consumes
// it and cross-checks the struct's json tags against the fixture's keys in
// both directions: a client field the server never serves (phantom) fails
// immediately, and a server field the client doesn't know about fails on the
// next fixture refresh unless it is explicitly skip-listed.
//
// The stats summary types (StatsSummary, AppLogStatsSummary and their
// buckets) have no upstream fixture yet, so they are not covered here.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// jsonTags returns the set of json tag names declared on struct type typ.
func jsonTags(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()
	tags := make(map[string]bool, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			t.Fatalf("field %s has no usable json tag", field.Name)
		}
		tags[name] = true
	}
	return tags
}

func toSet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set
}

func mustUnmarshal(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("unmarshal into %T: %v", v, err)
	}
}

func TestGoldenContract(t *testing.T) {
	// Ingest provenance fields the client omits by choice (web-UI/admin
	// territory, not live-tail); decode is unaffected.
	applogSkip := []string{"source_ip", "api_key_id"}

	tests := []struct {
		fixture      string
		typ          reflect.Type // struct mirroring the fixture's top-level object
		skipKeys     []string     // fixture keys the client omits by choice
		optionalTags []string     // omitempty tags legitimately absent from this fixture
		check        func(t *testing.T, raw []byte)
	}{
		{
			fixture: "srvlog_event.json",
			typ:     reflect.TypeOf(SrvlogEvent{}),
			check: func(t *testing.T, raw []byte) {
				var e SrvlogEvent
				mustUnmarshal(t, raw, &e)
				if e.ID != 1001 || e.Hostname != "web01.example.net" || e.Severity != 4 {
					t.Errorf("sentinel mismatch: id=%d hostname=%q severity=%d", e.ID, e.Hostname, e.Severity)
				}
				if e.StructuredData == nil || e.RawMessage == nil {
					t.Error("structured_data and raw_message should be non-nil")
				}
			},
		},
		{
			fixture:      "srvlog_event_nil_fields.json",
			typ:          reflect.TypeOf(SrvlogEvent{}),
			optionalTags: []string{"structured_data", "raw_message"},
			check: func(t *testing.T, raw []byte) {
				var e SrvlogEvent
				mustUnmarshal(t, raw, &e)
				if e.StructuredData != nil || e.RawMessage != nil {
					t.Error("structured_data and raw_message should be nil")
				}
				if e.Message == "" {
					t.Error("message should be set")
				}
			},
		},
		{
			fixture: "netlog_event.json",
			typ:     reflect.TypeOf(NetlogEvent{}),
			check: func(t *testing.T, raw []byte) {
				var e NetlogEvent
				mustUnmarshal(t, raw, &e)
				if e.ID != 2001 || e.FacilityLabel != "local7" {
					t.Errorf("sentinel mismatch: id=%d facility_label=%q", e.ID, e.FacilityLabel)
				}
				if e.StructuredData == nil {
					t.Error("structured_data should be non-nil")
				}
			},
		},
		{
			fixture:      "netlog_event_nil_fields.json",
			typ:          reflect.TypeOf(NetlogEvent{}),
			optionalTags: []string{"structured_data", "raw_message"},
			check: func(t *testing.T, raw []byte) {
				var e NetlogEvent
				mustUnmarshal(t, raw, &e)
				if e.StructuredData != nil || e.RawMessage != nil {
					t.Error("structured_data and raw_message should be nil")
				}
			},
		},
		{
			fixture:      "applog_event.json",
			typ:          reflect.TypeOf(AppLogEvent{}),
			skipKeys:     applogSkip,
			optionalTags: []string{"attrs_truncated"},
			check: func(t *testing.T, raw []byte) {
				var e AppLogEvent
				mustUnmarshal(t, raw, &e)
				if e.ID != 3001 || e.Level != "ERROR" || e.Service != "billing-api" {
					t.Errorf("sentinel mismatch: id=%d level=%q service=%q", e.ID, e.Level, e.Service)
				}
				if e.AttrsTruncated || string(e.Attrs) == "null" {
					t.Error("attrs should be present and not truncated")
				}
			},
		},
		{
			fixture:  "applog_event_attrs_truncated.json",
			typ:      reflect.TypeOf(AppLogEvent{}),
			skipKeys: applogSkip,
			check: func(t *testing.T, raw []byte) {
				var e AppLogEvent
				mustUnmarshal(t, raw, &e)
				if !e.AttrsTruncated {
					t.Error("attrs_truncated should be true")
				}
				if string(e.Attrs) != "null" {
					t.Errorf("attrs should be null, got %s", e.Attrs)
				}
			},
		},
		{
			fixture:      "applog_event_nil_fields.json",
			typ:          reflect.TypeOf(AppLogEvent{}),
			skipKeys:     applogSkip,
			optionalTags: []string{"attrs_truncated"},
			check: func(t *testing.T, raw []byte) {
				var e AppLogEvent
				mustUnmarshal(t, raw, &e)
				if e.Component != "" || e.Source != "" || e.AttrsTruncated {
					t.Errorf("zero-value mismatch: component=%q source=%q truncated=%v", e.Component, e.Source, e.AttrsTruncated)
				}
			},
		},
		{
			fixture: "list_envelope.json",
			typ:     reflect.TypeOf(ListResponse[SrvlogEvent]{}),
			check: func(t *testing.T, raw []byte) {
				var e ListResponse[SrvlogEvent]
				mustUnmarshal(t, raw, &e)
				if len(e.Data) != 2 || !e.HasMore || e.Cursor == nil {
					t.Fatalf("envelope mismatch: len=%d has_more=%v cursor=%v", len(e.Data), e.HasMore, e.Cursor)
				}
				if e.Data[1].ID != 1002 {
					t.Errorf("data[1].id = %d, want 1002", e.Data[1].ID)
				}
			},
		},
		{
			fixture:      "list_envelope_last_page.json",
			typ:          reflect.TypeOf(ListResponse[SrvlogEvent]{}),
			optionalTags: []string{"cursor"},
			check: func(t *testing.T, raw []byte) {
				var e ListResponse[SrvlogEvent]
				mustUnmarshal(t, raw, &e)
				if len(e.Data) != 0 || e.HasMore || e.Cursor != nil {
					t.Errorf("last page mismatch: len=%d has_more=%v cursor=%v", len(e.Data), e.HasMore, e.Cursor)
				}
			},
		},
		{
			fixture: "detail_envelope.json",
			typ:     reflect.TypeOf(ItemResponse[SrvlogEvent]{}),
			check: func(t *testing.T, raw []byte) {
				var e ItemResponse[SrvlogEvent]
				mustUnmarshal(t, raw, &e)
				if e.Data.ID != 1001 {
					t.Errorf("data.id = %d, want 1001", e.Data.ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "golden", tt.fixture))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var obj map[string]json.RawMessage
			mustUnmarshal(t, raw, &obj)

			tags := jsonTags(t, tt.typ)
			skip := toSet(tt.skipKeys)
			optional := toSet(tt.optionalTags)

			for key := range obj {
				if !tags[key] && !skip[key] {
					t.Errorf("fixture key %q has no client field: new server field? extend the struct or skip-list it", key)
				}
			}
			for tag := range tags {
				if _, ok := obj[tag]; !ok && !optional[tag] {
					t.Errorf("client field %q missing from fixture: phantom field?", tag)
				}
			}

			tt.check(t, raw)
		})
	}
}

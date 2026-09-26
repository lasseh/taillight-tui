package srvlog

import (
	"net/url"
	"strconv"

	"github.com/lasseh/taillight-tui/internal/tui/component"
	"github.com/lasseh/taillight-tui/internal/tui/view/logview"
)

// Query params accepted by the srvlog and netlog SSE endpoints, used as popup
// field keys and meta keys too.
const (
	paramHostname    = "hostname"
	paramProgramname = "programname"
	paramSeverityMax = "severity_max"
	paramFacility    = "facility"
)

// FilterModel is the srvlog filter: the fields accepted by the srvlog SSE
// endpoint beyond search. Metadata (hosts/programs) loaded from the API feeds
// the popup's typeahead.
type FilterModel struct {
	hostname    string
	program     string
	severityMax int // -1 = unset
	facility    int // -1 = unset
	hosts       []string
	programs    []string
}

func newFilter() *FilterModel {
	return &FilterModel{severityMax: -1, facility: -1}
}

// Meta keys typeahead suggestions for SetMeta.
func Meta(hosts, programs []string) map[string][]string {
	return map[string][]string{paramHostname: hosts, paramProgramname: programs}
}

// SetMeta implements logview.Filter.
func (f *FilterModel) SetMeta(meta map[string][]string) {
	f.hosts = meta[paramHostname]
	f.programs = meta[paramProgramname]
}

// Params implements logview.Filter.
func (f *FilterModel) Params() url.Values {
	v := url.Values{}
	if f.hostname != "" {
		v.Set(paramHostname, f.hostname)
	}
	if f.program != "" {
		v.Set(paramProgramname, f.program)
	}
	if f.severityMax >= 0 {
		v.Set(paramSeverityMax, strconv.Itoa(f.severityMax))
	}
	if f.facility >= 0 {
		v.Set(paramFacility, strconv.Itoa(f.facility))
	}
	return v
}

// Pills implements logview.Filter.
func (f *FilterModel) Pills(params url.Values) []string {
	var pills []string
	for _, p := range []struct{ key, label string }{
		{paramHostname, "host="},
		{paramProgramname, "prog="},
		{paramSeverityMax, "sev≤"},
		{paramFacility, "fac="},
	} {
		if v := params.Get(p.key); v != "" {
			pills = append(pills, p.label+v)
		}
	}
	return pills
}

// Apply implements logview.Filter.
func (f *FilterModel) Apply(values map[string]string) {
	if v, ok := values[paramHostname]; ok {
		f.hostname = v
	}
	if v, ok := values[paramProgramname]; ok {
		f.program = v
	}
	if v, ok := values[paramSeverityMax]; ok {
		f.severityMax = atoiOrUnset(v)
	}
	if v, ok := values[paramFacility]; ok {
		f.facility = atoiOrUnset(v)
	}
}

// atoiOrUnset parses a dropdown value; empty or invalid means unset (-1).
func atoiOrUnset(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

// Fields implements logview.Filter.
func (f *FilterModel) Fields() []component.Field {
	sevVal := ""
	if f.severityMax >= 0 {
		sevVal = strconv.Itoa(f.severityMax)
	}
	facVal := ""
	if f.facility >= 0 {
		facVal = strconv.Itoa(f.facility)
	}
	return []component.Field{
		{
			Key:         paramHostname,
			Label:       "Hostname",
			Kind:        component.FieldText,
			Suggestions: f.hosts,
			Value:       f.hostname,
		},
		{
			Key:         paramProgramname,
			Label:       "Program",
			Kind:        component.FieldText,
			Suggestions: f.programs,
			Value:       f.program,
		},
		{
			Key:     paramSeverityMax,
			Label:   "Severity",
			Kind:    component.FieldDropdown,
			Options: severityOptions(),
			Value:   sevVal,
		},
		{
			Key:     paramFacility,
			Label:   "Facility",
			Kind:    component.FieldDropdown,
			Options: facilityOptions(),
			Value:   facVal,
		},
	}
}

// severityOptions is the shared syslog severity list used by srvlog/netlog.
// severity_max semantics: "WARN" matches WARN + higher (lower number).
func severityOptions() []component.Option {
	return []component.Option{
		{Value: "", Label: "any"},
		{Value: "0", Label: "EMERG (0)"},
		{Value: "1", Label: "ALERT (1)"},
		{Value: "2", Label: "CRIT (2)"},
		{Value: "3", Label: "ERR (3)"},
		{Value: "4", Label: "WARNING (4)"},
		{Value: "5", Label: "NOTICE (5)"},
		{Value: "6", Label: "INFO (6)"},
		{Value: "7", Label: "DEBUG (7)"},
	}
}

// facilityOptions lists the well-known syslog facilities (0-23). Labels use
// the standard RFC 5424 names.
func facilityOptions() []component.Option {
	names := []string{
		"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
		"uucp", "cron", "authpriv", "ftp", "ntp", "security", "console", "clock",
		"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7",
	}
	opts := make([]component.Option, 0, len(names)+1)
	opts = append(opts, component.Option{Value: "", Label: "any"})
	for i, n := range names {
		opts = append(opts, component.Option{
			Value: strconv.Itoa(i),
			Label: n + " (" + strconv.Itoa(i) + ")",
		})
	}
	return opts
}

// Ensure srvlog FilterModel satisfies logview.Filter at compile time.
var _ logview.Filter = (*FilterModel)(nil)

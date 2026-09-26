package applog

import (
	"net/url"

	"github.com/lasseh/taillight-tui/internal/tui/component"
	"github.com/lasseh/taillight-tui/internal/tui/view/logview"
)

// Query params accepted by the applog SSE endpoint, used as popup field keys
// and meta keys too.
const (
	paramService   = "service"
	paramComponent = "component"
	paramHost      = "host"
	paramLevel     = "level"
)

// FilterModel is the applog filter: the fields accepted by the applog SSE
// endpoint beyond search. Metadata for typeahead is loaded from the API.
type FilterModel struct {
	service    string
	component  string
	host       string
	level      string // minimum level: WARN → WARN + ERROR + FATAL
	services   []string
	components []string
	hosts      []string
}

func newFilter() *FilterModel {
	return &FilterModel{}
}

// Meta keys typeahead suggestions for SetMeta.
func Meta(services, components, hosts []string) map[string][]string {
	return map[string][]string{paramService: services, paramComponent: components, paramHost: hosts}
}

// SetMeta implements logview.Filter.
func (f *FilterModel) SetMeta(meta map[string][]string) {
	f.services = meta[paramService]
	f.components = meta[paramComponent]
	f.hosts = meta[paramHost]
}

// Params implements logview.Filter.
func (f *FilterModel) Params() url.Values {
	v := url.Values{}
	if f.service != "" {
		v.Set(paramService, f.service)
	}
	if f.component != "" {
		v.Set(paramComponent, f.component)
	}
	if f.host != "" {
		v.Set(paramHost, f.host)
	}
	if f.level != "" {
		v.Set(paramLevel, f.level)
	}
	return v
}

// Pills implements logview.Filter.
func (f *FilterModel) Pills(params url.Values) []string {
	var pills []string
	for _, p := range []struct{ key, label string }{
		{paramHost, "host="},
		{paramService, "svc="},
		{paramComponent, "comp="},
		{paramLevel, "lvl="},
	} {
		if v := params.Get(p.key); v != "" {
			pills = append(pills, p.label+v)
		}
	}
	return pills
}

// Apply implements logview.Filter.
func (f *FilterModel) Apply(values map[string]string) {
	if v, ok := values[paramService]; ok {
		f.service = v
	}
	if v, ok := values[paramComponent]; ok {
		f.component = v
	}
	if v, ok := values[paramHost]; ok {
		f.host = v
	}
	if v, ok := values[paramLevel]; ok {
		f.level = v
	}
}

// Fields implements logview.Filter.
func (f *FilterModel) Fields() []component.Field {
	return []component.Field{
		{
			Key:         paramService,
			Label:       "Service",
			Kind:        component.FieldText,
			Suggestions: f.services,
			Value:       f.service,
		},
		{
			Key:         paramComponent,
			Label:       "Component",
			Kind:        component.FieldText,
			Suggestions: f.components,
			Value:       f.component,
		},
		{
			Key:         paramHost,
			Label:       "Host",
			Kind:        component.FieldText,
			Suggestions: f.hosts,
			Value:       f.host,
		},
		{
			Key:     paramLevel,
			Label:   "Level",
			Kind:    component.FieldDropdown,
			Options: levelOptions(),
			Value:   f.level,
		},
	}
}

// levelOptions lists applog levels from most-severe to least-severe. Selecting
// "WARN" matches WARN + ERROR + FATAL (the API uses min-level semantics).
func levelOptions() []component.Option {
	return []component.Option{
		{Value: "", Label: "any"},
		{Value: "FATAL", Label: "FATAL"},
		{Value: "ERROR", Label: "ERROR"},
		{Value: "WARN", Label: "WARN"},
		{Value: "INFO", Label: "INFO"},
		{Value: "DEBUG", Label: "DEBUG"},
	}
}

var _ logview.Filter = (*FilterModel)(nil)

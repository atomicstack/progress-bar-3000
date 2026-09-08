package format

import "testing"

type recordingResolver struct {
	widths map[string]int
	values map[string]string
}

func (r *recordingResolver) Resolve(name string, width int) string {
	if r.widths != nil {
		r.widths[name] = width
	}
	if r.values != nil {
		if value, ok := r.values[name]; ok {
			return value
		}
	}
	return ""
}

func TestTemplateRenderResolvesWidthAwareTokens(t *testing.T) {
	tmpl, err := Parse("%8{bar-only} %{phase} %%")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	resolver := &recordingResolver{
		widths: map[string]int{},
		values: map[string]string{
			"bar-only": "########",
			"phase":    "done",
		},
	}

	got := tmpl.Render(resolver)
	want := "######## done %"
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}

	if gotWidth := resolver.widths["bar-only"]; gotWidth != 8 {
		t.Fatalf("width for bar-only = %d, want 8", gotWidth)
	}
	if gotWidth := resolver.widths["phase"]; gotWidth != 0 {
		t.Fatalf("width for phase = %d, want 0", gotWidth)
	}
}

func TestTemplateRenderSupportsMetaKeys(t *testing.T) {
	tmpl, err := Parse("%{meta:branch}")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	resolver := &recordingResolver{
		widths: map[string]int{},
		values: map[string]string{
			"meta:branch": "main",
		},
	}

	got := tmpl.Render(resolver)
	want := "main"
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestTemplateParseRejectsMalformedTemplates(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "bare percent", in: "%"},
		{name: "open brace", in: "%{"},
		{name: "invalid alias", in: "%8x"},
		{name: "empty braced token", in: "%{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.in); err == nil {
				t.Fatalf("Parse(%q) error = nil, want non-nil", tt.in)
			}
		})
	}
}

func TestTemplateParseResolvesAliasTokens(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		token string
	}{
		{name: "progress", in: "%p", token: "progress"},
		{name: "timer", in: "%t", token: "timer"},
		{name: "eta", in: "%e", token: "eta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.in, err)
			}

			resolver := &recordingResolver{
				values: map[string]string{
					tt.token: "value",
				},
			}

			if got := tmpl.Render(resolver); got != "value" {
				t.Fatalf("Render() = %q, want %q", got, "value")
			}
		})
	}
}

func TestTemplateTokenWidth(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		token     string
		wantWidth int
		wantOK    bool
	}{
		{name: "absent", in: "%{percent} done", token: "phases", wantWidth: 0, wantOK: false},
		{name: "present without width", in: "%{bar-only} %{phases}", token: "phases", wantWidth: 0, wantOK: true},
		{name: "present with width", in: "%30{phases}", token: "phases", wantWidth: 30, wantOK: true},
		{name: "first occurrence wins", in: "%12{phases} %40{phases}", token: "phases", wantWidth: 12, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.in, err)
			}
			width, ok := tmpl.TokenWidth(tt.token)
			if width != tt.wantWidth || ok != tt.wantOK {
				t.Fatalf("TokenWidth(%q) = (%d, %v), want (%d, %v)", tt.token, width, ok, tt.wantWidth, tt.wantOK)
			}
		})
	}
}

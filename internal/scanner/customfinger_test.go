package scanner

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"cscan/internal/model"
)

func TestExtractQuotedValueKeepsInnerDoubleQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "daffodil footer",
			input: `"<div id="right_footer">Design & Development by Daffodil Software Ltd</div>"`,
			want:  `<div id="right_footer">Design & Development by Daffodil Software Ltd</div>`,
		},
		{
			name:  "energine footer",
			input: `"<div id="footer"><span class="copyright">Powered by <a href="http://energine.org">Energine</a><br/>"`,
			want:  `<div id="footer"><span class="copyright">Powered by <a href="http://energine.org">Energine</a><br/>`,
		},
		{
			name:  "escaped quote",
			input: `"id=\"swagger-ui"`,
			want:  `id="swagger-ui`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractQuotedValue(tt.input)
			if got != tt.want {
				t.Fatalf("extractQuotedValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchRuleBodyWithInnerDoubleQuotes(t *testing.T) {
	engine := NewCustomFingerprintEngine(nil)
	data := &FingerprintData{
		Body: `<html><div id="right_footer">Design & Development by Daffodil Software Ltd</div></html>`,
	}

	if !engine.matchRule(`body="<div id="right_footer">Design & Development by Daffodil Software Ltd</div>"`, data) {
		t.Fatal("expected full body rule with inner quotes to match")
	}

	if engine.matchRule(`body="<div id="footer"><span class="copyright">Powered by <a href="http://energine.org">Energine</a><br/>"`, data) {
		t.Fatal("expected unrelated full body rule not to match")
	}
}

func TestMatchWithIdPrefersBuiltinWappalyzerOverCustomDuplicate(t *testing.T) {
	engine := NewCustomFingerprintEngine([]*model.Fingerprint{
		{
			Name:    "Fireblade",
			Rule:    `title="Fireblade"`,
			Source:  "custom",
			Enabled: true,
		},
		{
			Name:      "Fireblade",
			HTML:      []string{`Fireblade`},
			Source:    "wappalyzer",
			IsBuiltin: true,
			Enabled:   true,
		},
	})

	matches := engine.MatchWithId(&FingerprintData{Title: "Fireblade", Body: "Fireblade"})
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(matches))
	}
	if matches[0].Source != "wappalyzer" {
		t.Fatalf("Source = %q, want wappalyzer", matches[0].Source)
	}
	if !matches[0].IsBuiltin {
		t.Fatal("expected builtin wappalyzer match to be preferred")
	}
}

func TestFormatAppWithSourcesUsesRealFingerprintSource(t *testing.T) {
	wappalyzerResult := &AppDetectionResult{
		Name:         "VentryShield",
		OriginalName: "VentryShield",
		Sources:      []string{"wappalyzer"},
	}
	if got := formatAppWithSources(wappalyzerResult); got != "VentryShield[wappalyzer]" {
		t.Fatalf("formatAppWithSources() = %q, want VentryShield[wappalyzer]", got)
	}

	customResult := &AppDetectionResult{
		Name:         "Daffodil-CRM",
		OriginalName: "Daffodil-CRM",
		Sources:      []string{"custom"},
		CustomIDs:    []string{"69f36180002636c8d5a5ebc0"},
	}
	if got := formatAppWithSources(customResult); got != "Daffodil-CRM[custom(69f36180002636c8d5a5ebc0)]" {
		t.Fatalf("formatAppWithSources() = %q, want custom id suffix", got)
	}
}

func TestFormatAppWithSourcesIncludesAllFourSources(t *testing.T) {
	result := &AppDetectionResult{
		Name:         "Kibana",
		OriginalName: "Kibana",
		Sources:      []string{"active", "custom", "wappalyzer", "httpx"},
		CustomIDs:    []string{"custom-id"},
		ActiveIDs:    []string{"active-id"},
	}
	want := "Kibana[httpx+wappalyzer+custom(custom-id)+active(active-id)]"
	if got := formatAppWithSources(result); got != want {
		t.Fatalf("formatAppWithSources() = %q, want %q", got, want)
	}
}

func TestMergeExistingAppDetectionsAddsMissingSourceSuffix(t *testing.T) {
	appResults := make(map[string]*AppDetectionResult)
	mergeExistingAppDetections(appResults, []string{"Elasticsearch Kibana"})
	result := appResults["elasticsearch kibana"]
	if result == nil {
		t.Fatal("expected app result")
	}
	if got := formatAppWithSources(result); got != "Elasticsearch Kibana[httpx]" {
		t.Fatalf("formatAppWithSources() = %q, want Elasticsearch Kibana[httpx]", got)
	}
}

func TestMergeActiveFingerprintAppCombinesExistingSources(t *testing.T) {
	asset := &Asset{App: []string{"Kibana[httpx+custom(custom-id)]"}}
	fp := &model.Fingerprint{Name: "Kibana", Enabled: true}
	fp.Id = primitive.NewObjectID()

	mergeActiveFingerprintApp(asset, fp)
	want := "Kibana[httpx+custom(custom-id)+active(" + fp.Id.Hex() + ")]"
	if len(asset.App) != 1 || asset.App[0] != want {
		t.Fatalf("asset.App = %#v, want %#v", asset.App, []string{want})
	}
}

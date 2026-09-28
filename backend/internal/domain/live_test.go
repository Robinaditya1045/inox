package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/inox/inox/backend/internal/domain"
)

// The summary is served to every signed-in user, so it must carry nothing about
// where a feed actually comes from beyond the resolver's name.
func TestLiveChannelSummaryOmitsOperatorFields(t *testing.T) {
	ch := &domain.LiveChannel{
		ID:             "chan-1",
		MediaAssetID:   "asset-1",
		Slug:           "lofi",
		Title:          "Lofi Radio",
		Resolver:       domain.ResolverBrowser,
		SourceURL:      "https://secret.example/watch?id=1",
		ResolverConfig: map[string]any{"api_key": "shh"},
		Fallbacks:      []string{"https://fallback.example/live.m3u8"},
		Status:         domain.LiveStatusLive,
		IsDVR:          true,
		UpstreamURL:    "https://cdn.example/tokenised.m3u8",
		LastError:      "upstream 403",
	}

	raw, err := json.Marshal(ch.Summary())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"media_asset_id": "asset-1",
		"slug":           "lofi",
		"title":          "Lofi Radio",
		"resolver":       "browser",
		"status":         "live",
		"is_dvr":         true,
	}
	if len(got) != len(want) {
		t.Fatalf("summary has fields %v, want exactly %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

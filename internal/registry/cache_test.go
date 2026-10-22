package registry

import (
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	cache := Cache{
		"npm:fresh":   {Repo: "acme/fresh", At: now.Add(-24 * time.Hour)},
		"npm:private": {Repo: "", At: now.Add(-time.Hour)},
		"npm:stale":   {Repo: "acme/stale", At: now.Add(-8 * 24 * time.Hour)},
	}
	if repo, ok := cache.Lookup("npm:fresh", now); !ok || repo != "acme/fresh" {
		t.Errorf("fresh = %q %v", repo, ok)
	}
	if repo, ok := cache.Lookup("npm:private", now); !ok || repo != "" {
		t.Errorf("a remembered 'not on GitHub' should be a hit: %q %v", repo, ok)
	}
	if _, ok := cache.Lookup("npm:stale", now); ok {
		t.Error("stale entry should miss")
	}
	updated := cache.With(map[string]string{"npm:new": "acme/new"}, now)
	if _, kept := updated["npm:stale"]; kept || len(updated) != 3 || updated["npm:new"].Repo != "acme/new" {
		t.Errorf("With = %+v", updated)
	}
	if len(cache) != 3 {
		t.Error("With must not change the original cache")
	}
	if round := DecodeCache(updated.Encode()); len(round) != 3 || round["npm:new"].Repo != "acme/new" {
		t.Errorf("round trip = %+v", round)
	}
	if len(DecodeCache([]byte("garbage"))) != 0 {
		t.Error("garbage should decode to an empty cache")
	}
}

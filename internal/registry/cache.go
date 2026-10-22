package registry

import (
	"encoding/json"
	"time"
)

// cacheTTL is how long a registry answer is trusted. Packages rarely move repositories.
const cacheTTL = 7 * 24 * time.Hour

// Cache remembers registry answers between runs, so running deps again on the
// same project makes almost no requests. Keys are dependency Key()s.
type Cache map[string]CacheEntry

// CacheEntry is one remembered answer; Repo is "" for a package not on GitHub.
type CacheEntry struct {
	Repo string    `json:"repo"`
	At   time.Time `json:"at"`
}

// Lookup returns a fresh cached answer.
func (cache Cache) Lookup(key string, now time.Time) (string, bool) {
	entry, ok := cache[key]
	if !ok || now.Sub(entry.At) > cacheTTL {
		return "", false
	}
	return entry.Repo, true
}

// With returns a new cache holding the fresh entries of this one plus new answers.
// Stale entries are dropped so the file does not grow forever.
func (cache Cache) With(answers map[string]string, now time.Time) Cache {
	updated := Cache{}
	for key, entry := range cache {
		if now.Sub(entry.At) <= cacheTTL {
			updated[key] = entry
		}
	}
	for key, repo := range answers {
		updated[key] = CacheEntry{Repo: repo, At: now}
	}
	return updated
}

// DecodeCache reads a cache file's content; anything unreadable is an empty cache.
func DecodeCache(content []byte) Cache {
	cache := Cache{}
	if json.Unmarshal(content, &cache) != nil {
		return Cache{}
	}
	return cache
}

// Encode returns the cache as file content.
func (cache Cache) Encode() []byte {
	content, _ := json.Marshal(cache)
	return content
}

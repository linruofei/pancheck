package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"sync"
	"time"

	"PanCheck/internal/checker"
	"PanCheck/internal/model"
)

func cacheKey(link string) string {
	sum := sha1.Sum([]byte(link))
	return hex.EncodeToString(sum[:])
}

type CacheRepository interface {
	Get(ctx context.Context, link string) (*checker.CheckResult, error)
	Set(ctx context.Context, link string, result *checker.CheckResult, platform model.Platform, invalidTTL int, platformTTLMap map[model.Platform]int) error
	Delete(ctx context.Context, link string) error
	Close() error
	IsEnabled() bool
}

type CacheConfig struct {
	Enabled bool
}

type memoryCacheEntry struct {
	Link      string
	Result    checker.CheckResult
	QueryTime time.Time
}

type memoryCacheRepository struct {
	enabled bool
}

var memoryCacheStore = struct {
	sync.RWMutex
	links map[string]memoryCacheEntry
}{
	links: make(map[string]memoryCacheEntry),
}

var (
	cleanupMu         sync.RWMutex
	NextMemoryCleanup time.Time
)

func SetNextMemoryCleanup(t time.Time) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	NextMemoryCleanup = t
}

func GetNextMemoryCleanup() time.Time {
	cleanupMu.RLock()
	defer cleanupMu.RUnlock()
	return NextMemoryCleanup
}

func NewCacheRepository(config CacheConfig) (CacheRepository, error) {
	return &memoryCacheRepository{enabled: config.Enabled}, nil
}

func (r *memoryCacheRepository) IsEnabled() bool {
	return r.enabled
}

func (r *memoryCacheRepository) Get(ctx context.Context, link string) (*checker.CheckResult, error) {
	if !r.IsEnabled() {
		return nil, nil
	}

	key := cacheKey(link)
	now := time.Now()

	memoryCacheStore.Lock()
	defer memoryCacheStore.Unlock()

	entry, ok := memoryCacheStore.links[key]
	if !ok {
		return nil, nil
	}
	entry.QueryTime = now
	memoryCacheStore.links[key] = entry
	result := entry.Result
	return &result, nil
}

func (r *memoryCacheRepository) Set(ctx context.Context, link string, result *checker.CheckResult, platform model.Platform, invalidTTL int, platformTTLMap map[model.Platform]int) error {
	if !r.IsEnabled() || result == nil || !result.Valid {
		return nil
	}

	memoryCacheStore.Lock()
	defer memoryCacheStore.Unlock()

	memoryCacheStore.links[cacheKey(link)] = memoryCacheEntry{
		Link:      link,
		Result:    *result,
		QueryTime: time.Now(),
	}
	return nil
}

func (r *memoryCacheRepository) Delete(ctx context.Context, link string) error {
	if !r.IsEnabled() {
		return nil
	}

	memoryCacheStore.Lock()
	defer memoryCacheStore.Unlock()

	delete(memoryCacheStore.links, cacheKey(link))
	return nil
}

func (r *memoryCacheRepository) Close() error {
	return nil
}

func CleanupCheckedLinks(olderThan time.Duration) int {
	if olderThan <= 0 {
		return 0
	}
	cutoff := time.Now().Add(-olderThan)
	deleted := 0

	memoryCacheStore.Lock()
	defer memoryCacheStore.Unlock()

	for key, entry := range memoryCacheStore.links {
		if entry.QueryTime.Before(cutoff) {
			delete(memoryCacheStore.links, key)
			deleted++
		}
	}
	return deleted
}

type CachedLinkEntry struct {
	Link      string    `json:"link"`
	Valid     bool      `json:"valid"`
	QueryTime time.Time `json:"query_time"`
}

func ListCheckedLinks() []CachedLinkEntry {
	memoryCacheStore.RLock()
	defer memoryCacheStore.RUnlock()

	entries := make([]CachedLinkEntry, 0, len(memoryCacheStore.links))
	for _, entry := range memoryCacheStore.links {
		entries = append(entries, CachedLinkEntry{
			Link:      entry.Link,
			Valid:     entry.Result.Valid,
			QueryTime: entry.QueryTime,
		})
	}
	return entries
}

func DeleteCheckedLink(link string) bool {
	memoryCacheStore.Lock()
	defer memoryCacheStore.Unlock()

	key := cacheKey(link)
	if _, ok := memoryCacheStore.links[key]; ok {
		delete(memoryCacheStore.links, key)
		return true
	}
	return false
}

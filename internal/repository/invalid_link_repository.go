package repository

import (
	"crypto/sha1"
	"encoding/hex"
	"sync"
	"time"

	"PanCheck/internal/model"
)

type InvalidLinkRepository struct{}

var invalidLinkMemoryStore = struct {
	sync.RWMutex
	nextID uint
	links  map[string]model.InvalidLink
}{
	links: make(map[string]model.InvalidLink),
}

func NewInvalidLinkRepository() *InvalidLinkRepository {
	return &InvalidLinkRepository{}
}

func linkHash(link string) string {
	sum := sha1.Sum([]byte(link))
	return hex.EncodeToString(sum[:])
}

func (r *InvalidLinkRepository) FindByLinks(links []string) ([]model.InvalidLink, error) {
	now := time.Now()
	results := make([]model.InvalidLink, 0, len(links))

	invalidLinkMemoryStore.Lock()
	defer invalidLinkMemoryStore.Unlock()

	for _, link := range links {
		key := linkHash(link)
		invalidLink, ok := invalidLinkMemoryStore.links[key]
		if !ok {
			continue
		}
		invalidLink.QueryTime = now
		invalidLinkMemoryStore.links[key] = invalidLink
		results = append(results, invalidLink)
	}
	return results, nil
}

func (r *InvalidLinkRepository) CreateOrUpdate(invalidLink *model.InvalidLink) error {
	now := time.Now()
	if invalidLink.CreatedAt.IsZero() {
		invalidLink.CreatedAt = now
	}
	invalidLink.QueryTime = now

	invalidLinkMemoryStore.Lock()
	defer invalidLinkMemoryStore.Unlock()

	key := linkHash(invalidLink.Link)
	if existing, ok := invalidLinkMemoryStore.links[key]; ok {
		invalidLink.ID = existing.ID
		invalidLink.CreatedAt = existing.CreatedAt
	}
	if invalidLink.ID == 0 {
		invalidLinkMemoryStore.nextID++
		invalidLink.ID = invalidLinkMemoryStore.nextID
	}
	invalidLinkMemoryStore.links[key] = *invalidLink
	return nil
}

func (r *InvalidLinkRepository) Exists(link string) (bool, error) {
	links, err := r.FindByLinks([]string{link})
	if err != nil {
		return false, err
	}
	if len(links) == 0 {
		return false, nil
	}
	return !links[0].IsRateLimited, nil
}

func CleanupInvalidLinks(olderThan time.Duration) int {
	if olderThan <= 0 {
		return 0
	}
	cutoff := time.Now().Add(-olderThan)
	deleted := 0

	invalidLinkMemoryStore.Lock()
	defer invalidLinkMemoryStore.Unlock()

	for key, invalidLink := range invalidLinkMemoryStore.links {
		queryTime := invalidLink.QueryTime
		if queryTime.IsZero() {
			queryTime = invalidLink.CreatedAt
		}
		if queryTime.Before(cutoff) {
			delete(invalidLinkMemoryStore.links, key)
			deleted++
		}
	}
	return deleted
}

func ListAllInvalidLinks() []model.InvalidLink {
	invalidLinkMemoryStore.RLock()
	defer invalidLinkMemoryStore.RUnlock()

	entries := make([]model.InvalidLink, 0, len(invalidLinkMemoryStore.links))
	for _, invalidLink := range invalidLinkMemoryStore.links {
		entries = append(entries, invalidLink)
	}
	return entries
}

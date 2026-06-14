package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"PanCheck/internal/checker"
	"PanCheck/internal/model"
	"PanCheck/internal/repository"
	"PanCheck/pkg/cache"
	apphttp "PanCheck/pkg/http"
	"PanCheck/pkg/validator"
)

type RealtimeCheckResult struct {
	ValidLinks    []string
	PendingLinks  []string
	InvalidLinks  []model.InvalidLink
	TotalDuration *int64
}

type CheckerService struct {
	checkerFactory  *checker.CheckerFactory
	invalidLinkRepo *repository.InvalidLinkRepository
	settingsRepo    *repository.SettingsRepository
	cacheRepo       cache.CacheRepository
	invalidTTL      int
	platformTTLMap  map[model.Platform]int
	ttlMu           sync.RWMutex
}

func NewCheckerService(factory *checker.CheckerFactory, cacheRepo cache.CacheRepository, invalidTTL int, platformTTLMap map[model.Platform]int) *CheckerService {
	return &CheckerService{
		checkerFactory:  factory,
		invalidLinkRepo: repository.NewInvalidLinkRepository(),
		settingsRepo:    repository.NewSettingsRepository(),
		cacheRepo:       cacheRepo,
		invalidTTL:      invalidTTL,
		platformTTLMap:  platformTTLMap,
	}
}

func (s *CheckerService) UpdateTTLConfig(invalidTTL int, platformTTLMap map[model.Platform]int) {
	s.ttlMu.Lock()
	defer s.ttlMu.Unlock()
	s.invalidTTL = invalidTTL
	s.platformTTLMap = platformTTLMap
}

func (s *CheckerService) CheckRealtime(links []string) (*RealtimeCheckResult, error) {
	return s.checkRealtime(links, nil, true)
}

func (s *CheckerService) CheckRealtimeWithPlatformFilter(links []string, selectedPlatforms []model.Platform) (*RealtimeCheckResult, error) {
	selected := make(map[model.Platform]bool, len(selectedPlatforms))
	for _, platform := range selectedPlatforms {
		selected[platform] = true
	}
	return s.checkRealtime(links, selected, false)
}

func (s *CheckerService) checkRealtime(links []string, selected map[model.Platform]bool, markUnknownInvalid bool) (*RealtimeCheckResult, error) {
	startTime := time.Now()
	uniqueLinks := deduplicateLinks(links)
	log.Printf("Realtime check started with %d unique links", len(uniqueLinks))

	linkInfos := make([]validator.LinkInfo, 0, len(uniqueLinks))
	for _, link := range uniqueLinks {
		linkInfos = append(linkInfos, validator.ParseLink(link))
	}

	linksByPlatform := make(map[model.Platform][]string)
	unknownLinks := make([]string, 0)
	pendingLinks := make([]string, 0)
	disabledPlatformLinks := make([]string, 0)
	platformEnabledMap := s.loadPlatformEnabledMap()

	for _, info := range linkInfos {
		if info.Platform == model.PlatformUnknown {
			if markUnknownInvalid {
				unknownLinks = append(unknownLinks, info.Link)
			} else {
				pendingLinks = append(pendingLinks, info.Link)
			}
			continue
		}
		if selected != nil && !selected[info.Platform] {
			pendingLinks = append(pendingLinks, info.Link)
			continue
		}
		if !platformEnabledMap[info.Platform] {
			disabledPlatformLinks = append(disabledPlatformLinks, info.Link)
			continue
		}
		linksByPlatform[info.Platform] = append(linksByPlatform[info.Platform], info.Link)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	validLinks := make([]string, 0, len(disabledPlatformLinks))
	validLinks = append(validLinks, disabledPlatformLinks...)
	invalidLinks := make([]model.InvalidLink, 0)

	for _, link := range unknownLinks {
		invalidLinks = append(invalidLinks, model.InvalidLink{
			Link:          link,
			Platform:      model.PlatformUnknown,
			FailureReason: "无法识别网盘平台类型",
			IsRateLimited: false,
			CreatedAt:     time.Now(),
		})
	}

	for platform, platformLinks := range linksByPlatform {
		linkChecker, ok := s.checkerFactory.GetChecker(platform)
		if !ok {
			for _, link := range platformLinks {
				invalidLinks = append(invalidLinks, model.InvalidLink{
					Link:          link,
					Platform:      platform,
					FailureReason: "该平台检测器未实现",
					IsRateLimited: false,
					CreatedAt:     time.Now(),
				})
			}
			continue
		}

		concurrencyLimit := linkChecker.GetConcurrencyLimit()
		if concurrencyLimit <= 0 {
			concurrencyLimit = 5
		}

		wg.Add(1)
		go func(ch checker.LinkChecker, plinks []string, limit int) {
			defer wg.Done()
			s.checkLinksWithConcurrency(context.Background(), ch, plinks, limit, &mu, &validLinks, &invalidLinks)
		}(linkChecker, platformLinks, concurrencyLimit)
	}

	wg.Wait()

	for _, invalidLink := range invalidLinks {
		if err := s.invalidLinkRepo.CreateOrUpdate(&invalidLink); err != nil {
			log.Printf("Failed to save invalid link %s: %v", invalidLink.Link, err)
		}
	}

	duration := time.Since(startTime).Milliseconds()
	log.Printf("Realtime check completed, valid: %d, invalid: %d, pending: %d, duration: %dms",
		len(validLinks), len(invalidLinks), len(pendingLinks), duration)

	return &RealtimeCheckResult{
		ValidLinks:    validLinks,
		PendingLinks:  pendingLinks,
		InvalidLinks:  invalidLinks,
		TotalDuration: &duration,
	}, nil
}

func deduplicateLinks(links []string) []string {
	seen := make(map[string]bool, len(links))
	uniqueLinks := make([]string, 0, len(links))
	for _, link := range links {
		normalizedLink := strings.TrimSpace(link)
		if normalizedLink == "" || seen[normalizedLink] {
			continue
		}
		seen[normalizedLink] = true
		uniqueLinks = append(uniqueLinks, normalizedLink)
	}
	return uniqueLinks
}

func (s *CheckerService) checkLinksWithConcurrency(
	ctx context.Context,
	ch checker.LinkChecker,
	links []string,
	concurrencyLimit int,
	mu *sync.Mutex,
	validLinks *[]string,
	invalidLinks *[]model.InvalidLink,
) {
	platform := ch.GetPlatform()
	log.Printf("Checking %d links for platform %s with concurrency %d", len(links), platform, concurrencyLimit)

	sem := make(chan struct{}, concurrencyLimit)
	var wg sync.WaitGroup

	for _, link := range links {
		wg.Add(1)
		sem <- struct{}{}
		go func(l string) {
			defer func() {
				<-sem
				wg.Done()
			}()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered while checking link %s for platform %s: %v", l, platform, r)
				}
			}()

			linkInfo := validator.ParseLink(l)
			normalizedLink := linkInfo.Link

			result, fromCache := s.loadCachedCheckResult(ctx, normalizedLink, platform)
			var err error
			if result == nil {
				result, err = ch.Check(l)
			}

			if err != nil {
				s.appendErrorResult(ctx, ch, normalizedLink, result, err, mu, invalidLinks)
				return
			}

			if !fromCache && result != nil {
				s.cacheCheckResult(ctx, normalizedLink, platform, result)
			}

			if result == nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			if result.Valid {
				*validLinks = append(*validLinks, normalizedLink)
				return
			}
			*invalidLinks = append(*invalidLinks, model.InvalidLink{
				Link:          normalizedLink,
				Platform:      platform,
				FailureReason: result.FailureReason,
				CheckDuration: &result.Duration,
				IsRateLimited: result.IsRateLimited,
				CreatedAt:     time.Now(),
			})
		}(link)
	}

	wg.Wait()
}

func (s *CheckerService) loadCachedCheckResult(ctx context.Context, link string, platform model.Platform) (*checker.CheckResult, bool) {
	if s.cacheRepo != nil && s.cacheRepo.IsEnabled() {
		cachedResult, err := s.cacheRepo.Get(ctx, link)
		if err == nil && cachedResult != nil {
			log.Printf("Cache hit for link %s (platform %s)", link, platform)
			return cachedResult, true
		}
	}

	exists, err := s.invalidLinkRepo.Exists(link)
	if err != nil || !exists {
		return nil, false
	}
	invalidLinks, err := s.invalidLinkRepo.FindByLinks([]string{link})
	if err != nil || len(invalidLinks) == 0 {
		return nil, false
	}
	invalidLink := invalidLinks[0]
	var duration int64
	if invalidLink.CheckDuration != nil {
		duration = *invalidLink.CheckDuration
	}
	return &checker.CheckResult{
		Valid:         false,
		FailureReason: invalidLink.FailureReason,
		Duration:      duration,
		IsRateLimited: invalidLink.IsRateLimited,
	}, true
}

func (s *CheckerService) appendErrorResult(
	ctx context.Context,
	ch checker.LinkChecker,
	link string,
	result *checker.CheckResult,
	err error,
	mu *sync.Mutex,
	invalidLinks *[]model.InvalidLink,
) {
	duration := int64(0)
	isRateLimited := false
	if result != nil {
		duration = result.Duration
		isRateLimited = result.IsRateLimited
	}
	failureReason := fmt.Sprintf("检测错误: %v", err)
	if apphttp.IsRateLimitError(err) {
		isRateLimited = true
		failureReason = fmt.Sprintf("API 频率限制: %v", err)
	}

	mu.Lock()
	*invalidLinks = append(*invalidLinks, model.InvalidLink{
		Link:          link,
		Platform:      ch.GetPlatform(),
		FailureReason: failureReason,
		CheckDuration: &duration,
		IsRateLimited: isRateLimited,
		CreatedAt:     time.Now(),
	})
	mu.Unlock()

}

func (s *CheckerService) cacheCheckResult(ctx context.Context, link string, platform model.Platform, result *checker.CheckResult) {
	if result == nil || !result.Valid {
		return
	}
	s.ttlMu.RLock()
	invalidTTL := s.invalidTTL
	platformTTLMap := s.platformTTLMap
	s.ttlMu.RUnlock()
	if err := s.cacheRepo.Set(ctx, link, result, platform, invalidTTL, platformTTLMap); err != nil {
		log.Printf("Failed to cache result for link %s: %v", link, err)
	}
}

func (s *CheckerService) GetConcurrencyLimit(platform model.Platform) int {
	key := fmt.Sprintf("platform_concurrency_%s", platform.String())
	_, err := s.settingsRepo.GetByKey(key)
	if err != nil {
		if ch, ok := s.checkerFactory.GetChecker(platform); ok {
			return ch.GetConcurrencyLimit()
		}
		return 5
	}
	if ch, ok := s.checkerFactory.GetChecker(platform); ok {
		return ch.GetConcurrencyLimit()
	}
	return 5
}

func (s *CheckerService) loadPlatformEnabledMap() map[model.Platform]bool {
	enabledMap := make(map[model.Platform]bool)
	for _, platform := range model.AllPlatforms() {
		enabledMap[platform] = true
		key := fmt.Sprintf("platform_rate_config_%s", platform.String())
		setting, err := s.settingsRepo.GetByKey(key)
		if err != nil || setting == nil {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(setting.Value), &raw); err != nil {
			continue
		}
		rawEnabled, ok := raw["enabled"]
		if !ok {
			continue
		}

		var enabled bool
		if err := json.Unmarshal(rawEnabled, &enabled); err != nil {
			continue
		}
		enabledMap[platform] = enabled
	}
	return enabledMap
}

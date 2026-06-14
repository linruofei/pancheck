package handler

import (
	"net/http"
	"time"

	"PanCheck/internal/config"
	"PanCheck/internal/repository"
	"PanCheck/pkg/cache"

	"github.com/gin-gonic/gin"
)

type MemoryHandler struct{}

func NewMemoryHandler() *MemoryHandler {
	return &MemoryHandler{}
}

type memoryOverview struct {
	HistoryTTLMinutes      int       `json:"history_ttl_minutes"`
	CleanupIntervalMinutes int       `json:"cleanup_interval_minutes"`
	InvalidLinksTotal      int       `json:"invalid_links_total"`
	CheckedLinksTotal      int       `json:"checked_links_total"`
	NextCleanupAt          time.Time `json:"next_cleanup_at"`
}

type invalidLinkView struct {
	Link          string    `json:"link"`
	Platform      string    `json:"platform"`
	FailureReason string    `json:"failure_reason"`
	IsRateLimited bool      `json:"is_rate_limited"`
	CreatedAt     time.Time `json:"created_at"`
	QueryTime     time.Time `json:"query_time"`
	CleanupAt     time.Time `json:"cleanup_at"`
}

type checkedLinkView struct {
	Link      string    `json:"link"`
	Valid     bool      `json:"valid"`
	QueryTime time.Time `json:"query_time"`
	CleanupAt time.Time `json:"cleanup_at"`
}

func (h *MemoryHandler) GetOverview(c *gin.Context) {
	memCfg := config.AppConfig.Memory
	historyTTL := time.Duration(memCfg.HistoryTTLMinutes) * time.Minute

	invalidLinks := repository.ListAllInvalidLinks()
	checkedLinks := cache.ListCheckedLinks()

	now := time.Now()
	nextCleanup := now.Truncate(time.Minute).Add(time.Duration(memCfg.CleanupIntervalMinutes) * time.Minute)

	invalidViews := make([]invalidLinkView, 0, len(invalidLinks))
	for _, il := range invalidLinks {
		queryTime := il.QueryTime
		if queryTime.IsZero() {
			queryTime = il.CreatedAt
		}
		invalidViews = append(invalidViews, invalidLinkView{
			Link:          il.Link,
			Platform:      string(il.Platform),
			FailureReason: il.FailureReason,
			IsRateLimited: il.IsRateLimited,
			CreatedAt:     il.CreatedAt,
			QueryTime:     queryTime,
			CleanupAt:     queryTime.Add(historyTTL),
		})
	}

	checkedViews := make([]checkedLinkView, 0, len(checkedLinks))
	for _, cl := range checkedLinks {
		checkedViews = append(checkedViews, checkedLinkView{
			Link:      cl.Link,
			Valid:     cl.Valid,
			QueryTime: cl.QueryTime,
			CleanupAt: cl.QueryTime.Add(historyTTL),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"overview": memoryOverview{
				HistoryTTLMinutes:      memCfg.HistoryTTLMinutes,
				CleanupIntervalMinutes: memCfg.CleanupIntervalMinutes,
				InvalidLinksTotal:      len(invalidLinks),
				CheckedLinksTotal:      len(checkedLinks),
				NextCleanupAt:          nextCleanup,
			},
			"invalid_links": invalidViews,
			"checked_links": checkedViews,
		},
	})
}

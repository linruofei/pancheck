package handler

import (
	"net/http"

	"PanCheck/internal/model"
	"PanCheck/internal/service"
	"PanCheck/pkg/utils"

	"github.com/gin-gonic/gin"
)

type LinkHandler struct {
	linkService    *service.LinkService
	checkerService *service.CheckerService
}

func NewLinkHandler(linkService *service.LinkService, checkerService *service.CheckerService) *LinkHandler {
	return &LinkHandler{
		linkService:    linkService,
		checkerService: checkerService,
	}
}

func (h *LinkHandler) CheckLinks(c *gin.Context) {
	var req service.CheckLinksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deviceInfo := utils.ParseDeviceInfo(c.GetHeader("User-Agent"), c.GetHeader("Accept-Language"))
	resp, err := h.linkService.CheckLinks(&req, c.ClientIP(), deviceInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(req.SelectedPlatforms) > 0 && len(resp.PendingLinks) > 0 {
		var checkResult *service.RealtimeCheckResult
		if selectsAllPlatforms(req.SelectedPlatforms) {
			checkResult, err = h.checkerService.CheckRealtime(resp.PendingLinks)
		} else {
			checkResult, err = h.checkerService.CheckRealtimeWithPlatformFilter(resp.PendingLinks, req.SelectedPlatforms)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		resp.ValidLinks = checkResult.ValidLinks
		resp.PendingLinks = checkResult.PendingLinks
		resp.TotalDuration = checkResult.TotalDuration
		resp.InvalidLinks = append(resp.InvalidLinks, invalidLinkStrings(checkResult.InvalidLinks)...)
	}

	c.JSON(http.StatusOK, resp)
}

func selectsAllPlatforms(selectedPlatforms []model.Platform) bool {
	allPlatforms := model.AllPlatforms()
	if len(selectedPlatforms) < len(allPlatforms) {
		return false
	}

	selectedMap := make(map[model.Platform]bool, len(selectedPlatforms))
	for _, platform := range selectedPlatforms {
		selectedMap[platform] = true
	}
	for _, platform := range allPlatforms {
		if !selectedMap[platform] {
			return false
		}
	}
	return true
}

func invalidLinkStrings(invalidLinks []model.InvalidLink) []string {
	links := make([]string, 0, len(invalidLinks))
	for _, invalidLink := range invalidLinks {
		links = append(links, invalidLink.Link)
	}
	return links
}

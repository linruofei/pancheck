package service

import (
	"strings"

	"PanCheck/internal/model"
	"PanCheck/internal/repository"
	"PanCheck/pkg/utils"
	"PanCheck/pkg/validator"
)

type LinkService struct {
	invalidLinkRepo *repository.InvalidLinkRepository
}

func NewLinkService() *LinkService {
	return &LinkService{
		invalidLinkRepo: repository.NewInvalidLinkRepository(),
	}
}

type CheckLinksRequest struct {
	Links             []string         `json:"links" binding:"required"`
	SelectedPlatforms []model.Platform `json:"selected_platforms"`
}

type CheckLinksResponse struct {
	InvalidLinks       []string `json:"invalid_links"`
	PendingLinks       []string `json:"pending_links"`
	ValidLinks         []string `json:"valid_links"`
	TotalDuration      *int64   `json:"total_duration"`
	InvalidFormatCount int      `json:"invalid_format_count"`
	DuplicateCount     int      `json:"duplicate_count"`
}

func (s *LinkService) CheckLinks(req *CheckLinksRequest, _ string, _ utils.DeviceInfo) (*CheckLinksResponse, error) {
	linkMap := make(map[string]bool)
	uniqueLinks := make([]string, 0)
	duplicateCount := 0

	for _, link := range req.Links {
		normalizedLink := strings.TrimSpace(link)
		if normalizedLink == "" {
			continue
		}
		if linkMap[normalizedLink] {
			duplicateCount++
			continue
		}
		linkMap[normalizedLink] = true
		uniqueLinks = append(uniqueLinks, normalizedLink)
	}

	linkInfos := validator.ParseLinks(uniqueLinks)
	invalidFormatCount := countInvalidFormatLinks(uniqueLinks, linkInfos)
	if len(linkInfos) == 0 {
		return &CheckLinksResponse{
			InvalidLinks:       []string{},
			PendingLinks:       []string{},
			ValidLinks:         []string{},
			InvalidFormatCount: invalidFormatCount,
			DuplicateCount:     duplicateCount,
		}, nil
	}

	allLinks := make([]string, 0, len(linkInfos))
	for _, info := range linkInfos {
		allLinks = append(allLinks, info.Link)
	}

	knownInvalidLinks, err := s.invalidLinkRepo.FindByLinks(allLinks)
	if err != nil {
		return nil, err
	}

	invalidLinkMap := make(map[string]bool)
	invalidLinks := make([]string, 0, len(knownInvalidLinks))
	for _, invalidLink := range knownInvalidLinks {
		invalidLinkMap[invalidLink.Link] = true
		invalidLinks = append(invalidLinks, invalidLink.Link)
	}

	pendingLinks := make([]string, 0)
	for _, link := range allLinks {
		if !invalidLinkMap[link] {
			pendingLinks = append(pendingLinks, link)
		}
	}

	return &CheckLinksResponse{
		InvalidLinks:       invalidLinks,
		PendingLinks:       pendingLinks,
		ValidLinks:         []string{},
		InvalidFormatCount: invalidFormatCount,
		DuplicateCount:     duplicateCount,
	}, nil
}

func countInvalidFormatLinks(uniqueLinks []string, linkInfos []validator.LinkInfo) int {
	validLinkMap := make(map[string]bool, len(linkInfos))
	for _, info := range linkInfos {
		validLinkMap[info.Link] = true
	}

	invalidFormatCount := 0
	for _, link := range uniqueLinks {
		if !validLinkMap[link] {
			invalidFormatCount++
		}
	}
	return invalidFormatCount
}

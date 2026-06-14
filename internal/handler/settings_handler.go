package handler

import (
	"encoding/json"
	"net/http"

	"PanCheck/internal/config"
	"PanCheck/internal/model"
	"PanCheck/internal/repository"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsRepo      *repository.SettingsRepository
	reloadCheckerFunc func() error
}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{settingsRepo: repository.NewSettingsRepository()}
}

func (h *SettingsHandler) SetReloadCheckerFunc(fn func() error) {
	h.reloadCheckerFunc = fn
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	category := c.Query("category")
	var settings []model.Setting
	var err error
	if category != "" {
		settings, err = h.settingsRepo.GetByCategory(category)
	} else {
		settings, err = h.settingsRepo.GetAll()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": settings})
}

func (h *SettingsHandler) GetSetting(c *gin.Context) {
	setting, err := h.settingsRepo.GetByKey(c.Param("key"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "setting not found"})
		return
	}
	c.JSON(http.StatusOK, setting)
}

func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Value       string `json:"value" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	setting, err := h.settingsRepo.GetByKey(key)
	if err != nil {
		setting = &model.Setting{Key: key, Category: "checker"}
	}
	setting.Value = req.Value
	if req.Description != "" {
		setting.Description = req.Description
	}
	if err := h.settingsRepo.Update(setting); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, setting)
}

func (h *SettingsHandler) GetRateConfigSettings(c *gin.Context) {
	platforms := []string{"quark", "uc", "baidu", "tianyi", "pan123", "pan115", "aliyun", "xunlei", "cmcc"}
	result := make(map[string]*config.PlatformRateConfig)
	for _, platform := range platforms {
		defaultConfig := &config.PlatformRateConfig{
			Enabled:              true,
			Concurrency:          5,
			RequestDelayMs:       0,
			MaxRequestsPerSecond: 0,
			CacheTTLHours:        24,
		}
		setting, err := h.settingsRepo.GetByKey("platform_rate_config_" + platform)
		if err == nil && setting != nil {
			var rateConfig config.PlatformRateConfig
			if err := json.Unmarshal([]byte(setting.Value), &rateConfig); err == nil {
				if !jsonContainsEnabledField(setting.Value) {
					rateConfig.Enabled = true
				}
				result[platform] = &rateConfig
				continue
			}
		}
		result[platform] = defaultConfig
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *SettingsHandler) UpdateRateConfigSettings(c *gin.Context) {
	var req struct {
		Settings map[string]*config.PlatformRateConfig `json:"settings" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for platform, rateConfig := range req.Settings {
		if rateConfig.Concurrency < 1 || rateConfig.Concurrency > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": platform + " concurrency must be between 1 and 100"})
			return
		}
		if rateConfig.RequestDelayMs < 0 || rateConfig.RequestDelayMs > 10000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": platform + " request delay must be between 0 and 10000ms"})
			return
		}
		if rateConfig.MaxRequestsPerSecond < 0 || rateConfig.MaxRequestsPerSecond > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": platform + " max requests per second must be between 0 and 100"})
			return
		}
		if rateConfig.CacheTTLHours < 0 || rateConfig.CacheTTLHours > 720 {
			c.JSON(http.StatusBadRequest, gin.H{"error": platform + " cache TTL must be between 0 and 720 hours"})
			return
		}
	}

	updated := make(map[string]*config.PlatformRateConfig)
	for platform, rateConfig := range req.Settings {
		valueBytes, err := json.Marshal(rateConfig)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		key := "platform_rate_config_" + platform
		setting, err := h.settingsRepo.GetByKey(key)
		if err != nil {
			setting = &model.Setting{Key: key, Description: platform + " rate config", Category: "checker"}
		}
		setting.Value = string(valueBytes)
		if err := h.settingsRepo.Update(setting); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updated[platform] = rateConfig
	}

	if h.reloadCheckerFunc != nil {
		if err := h.reloadCheckerFunc(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "saved, but failed to reload checker config: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "saved", "data": updated})
}

func jsonContainsEnabledField(settingValue string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(settingValue), &raw); err != nil {
		return false
	}
	_, ok := raw["enabled"]
	return ok
}

func defaultMemoryConfig() config.MemoryConfig {
	return config.MemoryConfig{
		HistoryTTLMinutes:      2880,
		CleanupIntervalMinutes: 10,
	}
}

func (h *SettingsHandler) GetMemoryConfig(c *gin.Context) {
	setting, err := h.settingsRepo.GetByKey("memory_config")
	if err != nil || setting == nil {
		c.JSON(http.StatusOK, gin.H{"data": config.AppConfig.Memory})
		return
	}
	var memoryConfig config.MemoryConfig
	if err := json.Unmarshal([]byte(setting.Value), &memoryConfig); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": defaultMemoryConfig()})
		return
	}
	if memoryConfig.HistoryTTLMinutes == 0 {
		memoryConfig.HistoryTTLMinutes = 2880
	}
	if memoryConfig.CleanupIntervalMinutes == 0 {
		memoryConfig.CleanupIntervalMinutes = 10
	}
	c.JSON(http.StatusOK, gin.H{"data": memoryConfig})
}

func (h *SettingsHandler) UpdateMemoryConfig(c *gin.Context) {
	var req struct {
		Config config.MemoryConfig `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Config.HistoryTTLMinutes < 1 || req.Config.HistoryTTLMinutes > 43200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "history TTL must be between 1 and 43200 minutes"})
		return
	}
	if req.Config.CleanupIntervalMinutes < 1 || req.Config.CleanupIntervalMinutes > 1440 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cleanup interval must be between 1 and 1440 minutes"})
		return
	}

	config.AppConfig.Memory = req.Config
	valueBytes, err := json.Marshal(req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	setting, err := h.settingsRepo.GetByKey("memory_config")
	if err != nil {
		setting = &model.Setting{Key: "memory_config", Description: "Memory cleanup config", Category: "cache"}
	}
	setting.Value = string(valueBytes)
	if err := h.settingsRepo.Update(setting); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "saved", "data": req.Config})
}

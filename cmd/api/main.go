package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"PanCheck/internal/checker"
	"PanCheck/internal/config"
	"PanCheck/internal/handler"
	"PanCheck/internal/middleware"
	"PanCheck/internal/repository"
	"PanCheck/internal/service"
	"PanCheck/pkg/cache"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if err := config.Load(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	settingsRepo := repository.NewSettingsRepository()
	cacheRepo, err := cache.NewCacheRepository(cache.CacheConfig{Enabled: true})
	if err != nil {
		log.Printf("Failed to initialize memory cache: %v, continuing without cache", err)
	}
	if cacheRepo != nil {
		defer cacheRepo.Close()
	}

	factory := checker.NewCheckerFactory()
	initCheckers(factory, config.AppConfig.Checker)

	linkService := service.NewLinkService()
	checkerService := service.NewCheckerService(factory, cacheRepo, 0, nil)

	linkHandler := handler.NewLinkHandler(linkService, checkerService)
	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler()
	settingsHandler := handler.NewSettingsHandler()
	settingsHandler.SetReloadCheckerFunc(func() error {
		initCheckers(factory, config.AppConfig.Checker)
		return nil
	})
	memoryHandler := handler.NewMemoryHandler()

	startMemoryCleanup(config.AppConfig.Memory)

	if config.AppConfig.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(middleware.CORS(config.AppConfig.Server.CORSOrigins))

	r.Static("/assets", "./static/assets")
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.Static("/images", "./static/images")

	api := r.Group("/api/v1")
	{
		api.POST("/auth/login", authHandler.Login)
		api.GET("/health", healthHandler.Health)
		api.POST("/links/check", linkHandler.CheckLinks)

		apiAuth := api.Group("")
		apiAuth.Use(middleware.AuthMiddleware())
		{
			apiAuth.GET("/settings/rate-config", settingsHandler.GetRateConfigSettings)
			apiAuth.PUT("/settings/rate-config", settingsHandler.UpdateRateConfigSettings)
			apiAuth.GET("/settings/memory-config", settingsHandler.GetMemoryConfig)
			apiAuth.PUT("/settings/memory-config", settingsHandler.UpdateMemoryConfig)
			apiAuth.GET("/settings", settingsHandler.GetSettings)
			apiAuth.GET("/settings/:key", settingsHandler.GetSetting)
			apiAuth.PUT("/settings/:key", settingsHandler.UpdateSetting)

			apiAuth.GET("/memory/overview", memoryHandler.GetOverview)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}
		c.File("./static/index.html")
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.AppConfig.Server.Port),
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on port %d\n", config.AppConfig.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")

	_ = settingsRepo
}

func initCheckers(factory *checker.CheckerFactory, checkerConfig config.CheckerConfig) {
	timeout := time.Duration(checkerConfig.Timeout) * time.Second
	defaultConcurrency := checkerConfig.DefaultConcurrency
	settingsRepo := repository.NewSettingsRepository()

	getRateConfig := func(platform string) *config.PlatformRateConfig {
		key := fmt.Sprintf("platform_rate_config_%s", platform)
		setting, err := settingsRepo.GetByKey(key)
		if err == nil && setting != nil {
			var rateConfig config.PlatformRateConfig
			if err := json.Unmarshal([]byte(setting.Value), &rateConfig); err == nil {
				return &rateConfig
			}
		}
		return &config.PlatformRateConfig{
			Enabled:              true,
			Concurrency:          defaultConcurrency,
			RequestDelayMs:       0,
			MaxRequestsPerSecond: 0,
		}
	}

	registerChecker := func(platform string) {
		rateConfig := getRateConfig(platform)
		concurrency := rateConfig.Concurrency
		if concurrency <= 0 {
			concurrency = defaultConcurrency
		}

		var checkerInstance checker.LinkChecker
		switch platform {
		case "quark":
			checkerInstance = checker.NewQuarkChecker(concurrency, timeout)
		case "uc":
			checkerInstance = checker.NewUCChecker(concurrency, timeout)
		case "baidu":
			checkerInstance = checker.NewBaiduChecker(concurrency, timeout)
		case "tianyi":
			checkerInstance = checker.NewTianyiChecker(concurrency, timeout)
		case "pan123":
			checkerInstance = checker.NewPan123Checker(concurrency, timeout)
		case "pan115":
			checkerInstance = checker.NewPan115Checker(concurrency, timeout)
		case "aliyun":
			checkerInstance = checker.NewAliyunChecker(concurrency, timeout)
		case "xunlei":
			checkerInstance = checker.NewXunleiChecker(concurrency, timeout)
		case "cmcc":
			checkerInstance = checker.NewCMCCChecker(concurrency, timeout)
		default:
			return
		}

		if baseChecker, ok := checkerInstance.(interface {
			SetRateConfig(*config.PlatformRateConfig)
		}); ok {
			baseChecker.SetRateConfig(rateConfig)
		}
		factory.Register(checkerInstance)
	}

	for _, platform := range []string{"quark", "uc", "baidu", "tianyi", "pan123", "pan115", "aliyun", "xunlei", "cmcc"} {
		registerChecker(platform)
	}
}

func startMemoryCleanup(memoryConfig config.MemoryConfig) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		lastRun := time.Time{}
		var nextCleanup time.Time
		for range ticker.C {
			currentConfig := memoryConfig
			if config.AppConfig != nil {
				currentConfig = config.AppConfig.Memory
			}
			historyTTL := time.Duration(currentConfig.HistoryTTLMinutes) * time.Minute
			interval := time.Duration(currentConfig.CleanupIntervalMinutes) * time.Minute
			if historyTTL <= 0 {
				historyTTL = 48 * time.Hour
			}
			if interval <= 0 {
				interval = 10 * time.Minute
			}

			if nextCleanup.IsZero() || time.Now().After(nextCleanup) {
				lastRun = time.Now()
				nextCleanup = lastRun.Add(interval).Truncate(time.Minute)

				deletedInvalid := repository.CleanupInvalidLinks(historyTTL)
				deletedChecked := cache.CleanupCheckedLinks(historyTTL)
				if deletedInvalid > 0 || deletedChecked > 0 {
					log.Printf("Memory cleanup removed %d invalid links and %d checked link cache records", deletedInvalid, deletedChecked)
				}
			}
		}
	}()
}

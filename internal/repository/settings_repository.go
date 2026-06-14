package repository

import (
	"errors"
	"sort"
	"sync"
	"time"

	"PanCheck/internal/model"
)

type SettingsRepository struct{}

var settingsMemoryStore = struct {
	sync.RWMutex
	nextID   uint
	settings map[string]model.Setting
}{
	settings: make(map[string]model.Setting),
}

func NewSettingsRepository() *SettingsRepository {
	return &SettingsRepository{}
}

func (r *SettingsRepository) GetByKey(key string) (*model.Setting, error) {
	settingsMemoryStore.RLock()
	defer settingsMemoryStore.RUnlock()

	setting, ok := settingsMemoryStore.settings[key]
	if !ok {
		return nil, errors.New("setting not found")
	}
	return &setting, nil
}

func (r *SettingsRepository) GetOrCreate(key, value, description, category string) (*model.Setting, error) {
	setting, err := r.GetByKey(key)
	if err == nil {
		return setting, nil
	}

	now := time.Now()
	setting = &model.Setting{
		Key:         key,
		Value:       value,
		Description: description,
		Category:    category,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.Update(setting); err != nil {
		return nil, err
	}
	return setting, nil
}

func (r *SettingsRepository) Update(setting *model.Setting) error {
	settingsMemoryStore.Lock()
	defer settingsMemoryStore.Unlock()

	now := time.Now()
	existing, ok := settingsMemoryStore.settings[setting.Key]
	if ok {
		setting.ID = existing.ID
		setting.CreatedAt = existing.CreatedAt
		if setting.Description == "" {
			setting.Description = existing.Description
		}
		if setting.Category == "" {
			setting.Category = existing.Category
		}
	} else {
		settingsMemoryStore.nextID++
		setting.ID = settingsMemoryStore.nextID
		setting.CreatedAt = now
	}
	setting.UpdatedAt = now
	settingsMemoryStore.settings[setting.Key] = *setting
	return nil
}

func (r *SettingsRepository) GetByCategory(category string) ([]model.Setting, error) {
	settings, err := r.GetAll()
	if err != nil {
		return nil, err
	}

	filtered := make([]model.Setting, 0)
	for _, setting := range settings {
		if setting.Category == category {
			filtered = append(filtered, setting)
		}
	}
	return filtered, nil
}

func (r *SettingsRepository) GetAll() ([]model.Setting, error) {
	settingsMemoryStore.RLock()
	defer settingsMemoryStore.RUnlock()

	settings := make([]model.Setting, 0, len(settingsMemoryStore.settings))
	for _, setting := range settingsMemoryStore.settings {
		settings = append(settings, setting)
	}
	sort.Slice(settings, func(i, j int) bool {
		return settings[i].Key < settings[j].Key
	})
	return settings, nil
}

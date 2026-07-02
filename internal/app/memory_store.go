package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu         sync.RWMutex
	tasks      []Task
	aiProvider string
	authUsers  map[string]AuthUserRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks: []Task{
			{ID: 1, Title: "学习 Codex 本地开发", Done: true},
			{ID: 2, Title: "用 Git 管理版本", Done: false},
			{ID: 3, Title: "部署到腾讯云 Lighthouse", Done: false},
		},
		authUsers: map[string]AuthUserRecord{
			defaultAdminEmail: {
				ID:          1,
				Email:       defaultAdminEmail,
				Username:    "shalei",
				DisplayName: "",
				Role:        RoleAdmin,
				Enabled:     true,
			},
			"maming@soyoung.com": {
				ID:          2,
				Email:       "maming@soyoung.com",
				Username:    "maming",
				DisplayName: "",
				Role:        RoleAdmin,
				Enabled:     true,
			},
			"changwenxia@soyoung.com": {
				ID:          3,
				Email:       "changwenxia@soyoung.com",
				Username:    "changwenxia",
				DisplayName: "",
				Role:        RoleEditor,
				Enabled:     true,
			},
			"wangxiaofan@soyoung.com": {
				ID:          4,
				Email:       "wangxiaofan@soyoung.com",
				Username:    "wangxiaofan",
				DisplayName: "",
				Role:        RoleEditor,
				Enabled:     true,
			},
		},
	}
}

func (s *MemoryStore) Name() string {
	return "memory"
}

func (s *MemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *MemoryStore) ListTasks(ctx context.Context) ([]Task, error) {
	tasks := make([]Task, len(s.tasks))
	copy(tasks, s.tasks)
	return tasks, nil
}

func (s *MemoryStore) GetAIProvider(ctx context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aiProvider, nil
}

func (s *MemoryStore) SetAIProvider(ctx context.Context, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiProvider = NormalizeAIProvider(provider)
	return nil
}

func (s *MemoryStore) GetAuthUserByEmail(ctx context.Context, email string) (AuthUserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.authUsers[normalizeEmail(email)]
	if !ok {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	return user, nil
}

func (s *MemoryStore) UpdateAuthUserProfile(ctx context.Context, patch AuthUserPatch) (AuthUserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email := normalizeEmail(patch.Email)
	user, ok := s.authUsers[email]
	if !ok {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	if !user.Enabled {
		return user, nil
	}
	user.Username = firstNonEmpty(patch.Username, user.Username)
	user.DisplayName = firstNonEmpty(patch.DisplayName, user.DisplayName)
	user.FeishuUserID = firstNonEmpty(patch.FeishuUserID, user.FeishuUserID)
	user.Phone = firstNonEmpty(patch.Phone, user.Phone)
	now := time.Now()
	user.LastLoginAt = &now
	s.authUsers[email] = user
	return user, nil
}

func (s *MemoryStore) setAuthUserForTest(user AuthUserRecord) error {
	if strings.TrimSpace(user.Email) == "" {
		return errors.New("missing email")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authUsers[normalizeEmail(user.Email)] = user
	return nil
}

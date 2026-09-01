package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

type MemoryStore struct {
	mu                     sync.RWMutex
	tasks                  []Task
	aiProvider             string
	authUsers              map[string]AuthUserRecord
	monitorScopeCandidates []AuthUserResourceScope
	monitorScopesByUserID  map[int64][]AuthUserResourceScope
	auditLogs              []AuditLog
	nextAuditLogID         int64
	now                    func() time.Time
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
		monitorScopeCandidates: []AuthUserResourceScope{
			{StoreID: 30, City: "北京", Name: "北京保利实验室门店", ExternalOrgID: "10030"},
			{StoreID: 19, City: "上海", Name: "新氧青春诊所(上海陆家嘴店)", ExternalOrgID: "10019"},
		},
		monitorScopesByUserID: map[int64][]AuthUserResourceScope{},
		now:                   time.Now,
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

func (s *MemoryStore) CreateAuditLog(ctx context.Context, log AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAuditLogID++
	log.ID = s.nextAuditLogID
	log.CreatedAt = s.now()
	log.AssetLogicalKey = strings.TrimSpace(log.AssetLogicalKey)
	log.IPAddress = sanitizeAuditMetadata(log.IPAddress, 64)
	log.UserAgent = sanitizeAuditMetadata(log.UserAgent, 512)
	log.RequestID = sanitizeAuditMetadata(log.RequestID, 128)
	log.DetailJSON = sanitizeAuditDetail(log.DetailJSON)
	s.auditLogs = append(s.auditLogs, cloneAuditLog(log))
	return nil
}

// RecordAudit adapts the shared audit write port to the legacy app store API.
func (s *MemoryStore) RecordAudit(ctx context.Context, event AuditLog) error {
	return s.CreateAuditLog(ctx, event)
}

func (s *MemoryStore) ListAuditLogs(ctx context.Context, filter AuditLogFilter) (AuditLogPage, error) {
	filter, offset := normalizeAuditLogFilter(filter)
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]AuditLog, 0)
	for _, log := range s.auditLogs {
		if log.Action == internalAuditActionSnapshotRefreshPrepare {
			continue
		}
		if log.CreatedAt.Before(filter.StartAt) || !log.CreatedAt.Before(filter.EndAt) {
			continue
		}
		if filter.UserID != nil && (log.UserID == nil || *log.UserID != *filter.UserID) {
			continue
		}
		if filter.Action != "" && log.Action != filter.Action {
			continue
		}
		items = append(items, cloneAuditLog(log))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := len(items)
	if offset >= total {
		return AuditLogPage{Items: []AuditLog{}, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
	}
	end := offset + filter.PageSize
	if end > total {
		end = total
	}
	return AuditLogPage{Items: items[offset:end], Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
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
	s.attachMemoryMonitorScopes(&user)
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

func (s *MemoryStore) ListAuthUsers(ctx context.Context) ([]AuthUserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]AuthUserRecord, 0, len(s.authUsers))
	for _, user := range s.authUsers {
		s.attachMemoryMonitorScopes(&user)
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].Enabled != users[j].Enabled {
			return users[i].Enabled
		}
		return users[i].Email < users[j].Email
	})
	return users, nil
}

func (s *MemoryStore) CreateAuthUser(ctx context.Context, input AuthUserMutation) (AuthUserRecord, error) {
	email := normalizeEmail(input.Email)
	if email == "" {
		return AuthUserRecord{}, errors.New("missing email")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.authUsers[email]; ok {
		return AuthUserRecord{}, errors.New("auth user exists")
	}
	user := AuthUserRecord{
		ID:          int64(len(s.authUsers) + 1),
		Email:       email,
		Username:    strings.TrimSpace(input.Username),
		DisplayName: strings.TrimSpace(input.DisplayName),
		Role:        normalizeRole(input.Role),
		Enabled:     input.Enabled,
	}
	s.monitorScopesByUserID[user.ID] = s.monitorScopesForIDs(input.MonitorStoreScopeIDs)
	s.attachMemoryMonitorScopes(&user)
	s.authUsers[email] = user
	return user, nil
}

func (s *MemoryStore) CreateAuthUserWithAudit(ctx context.Context, input AuthUserMutation, event auditlog.AuditEvent) (AuthUserRecord, error) {
	user, err := s.CreateAuthUser(ctx, input)
	if err != nil {
		return AuthUserRecord{}, err
	}
	if event.EntityID == nil {
		event.EntityID = int64Pointer(user.ID)
	}
	if err := s.RecordAudit(ctx, event); err != nil {
		return AuthUserRecord{}, err
	}
	return user, nil
}

func (s *MemoryStore) UpdateAuthUser(ctx context.Context, id int64, input AuthUserMutation) (AuthUserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for email, user := range s.authUsers {
		if user.ID != id {
			continue
		}
		user.Username = strings.TrimSpace(input.Username)
		user.DisplayName = strings.TrimSpace(input.DisplayName)
		user.Role = normalizeRole(input.Role)
		user.Enabled = input.Enabled
		s.monitorScopesByUserID[user.ID] = s.monitorScopesForIDs(input.MonitorStoreScopeIDs)
		s.attachMemoryMonitorScopes(&user)
		s.authUsers[email] = user
		return user, nil
	}
	return AuthUserRecord{}, errAuthUserNotFound
}

func (s *MemoryStore) UpdateAuthUserWithAudit(ctx context.Context, id int64, input AuthUserMutation, event auditlog.AuditEvent) (AuthUserRecord, error) {
	user, err := s.UpdateAuthUser(ctx, id, input)
	if err != nil {
		return AuthUserRecord{}, err
	}
	if event.EntityID == nil {
		event.EntityID = int64Pointer(user.ID)
	}
	if err := s.RecordAudit(ctx, event); err != nil {
		return AuthUserRecord{}, err
	}
	return user, nil
}

func (s *MemoryStore) ListMonitorStoreScopeCandidates(ctx context.Context) ([]AuthUserResourceScope, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopes := make([]AuthUserResourceScope, len(s.monitorScopeCandidates))
	copy(scopes, s.monitorScopeCandidates)
	return scopes, nil
}

func (s *MemoryStore) GetUserMonitorStoreScopes(ctx context.Context, userID int64) ([]AuthUserResourceScope, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopes := s.monitorScopesByUserID[userID]
	result := make([]AuthUserResourceScope, len(scopes))
	copy(result, scopes)
	return result, nil
}

func (s *MemoryStore) CanUserViewMonitorStore(ctx context.Context, user AuthUserRecord, externalOrgID string) (bool, error) {
	if normalizeRole(user.Role) != RoleViewer {
		return true, nil
	}
	orgID := strings.TrimSpace(externalOrgID)
	if orgID == "" {
		return false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, scope := range s.monitorScopesByUserID[user.ID] {
		if strings.TrimSpace(scope.ExternalOrgID) == orgID {
			return true, nil
		}
	}
	return false, nil
}

func (s *MemoryStore) monitorScopesForIDs(ids []int64) []AuthUserResourceScope {
	idSet := map[int64]bool{}
	for _, id := range ids {
		if id > 0 {
			idSet[id] = true
		}
	}
	scopes := []AuthUserResourceScope{}
	for _, candidate := range s.monitorScopeCandidates {
		if idSet[candidate.StoreID] {
			scopes = append(scopes, candidate)
		}
	}
	return scopes
}

func (s *MemoryStore) attachMemoryMonitorScopes(user *AuthUserRecord) {
	if user == nil {
		return
	}
	scopes := s.monitorScopesByUserID[user.ID]
	user.MonitorStoreScopes = make([]AuthUserResourceScope, len(scopes))
	copy(user.MonitorStoreScopes, scopes)
	user.MonitorStoreScopeCount = len(scopes)
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

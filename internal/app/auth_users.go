package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	defaultAdminEmail = "shalei@soyoung.com"

	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"

	PermissionStoreRead  = "store:read"
	PermissionStoreWrite = "store:write"
	PermissionUserManage = "user:manage"
)

var errAuthUserNotFound = errors.New("auth user not found")

type AuthUserRecord struct {
	ID           int64
	Email        string
	Username     string
	DisplayName  string
	FeishuUserID string
	Phone        string
	Role         string
	Enabled      bool
	LastLoginAt  *time.Time
}

type AuthUserPatch struct {
	Email        string
	Username     string
	DisplayName  string
	FeishuUserID string
	Phone        string
}

type AuthUserMutation struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
}

type AuthUserStore interface {
	GetAuthUserByEmail(ctx context.Context, email string) (AuthUserRecord, error)
	UpdateAuthUserProfile(ctx context.Context, patch AuthUserPatch) (AuthUserRecord, error)
	ListAuthUsers(ctx context.Context) ([]AuthUserRecord, error)
	CreateAuthUser(ctx context.Context, input AuthUserMutation) (AuthUserRecord, error)
	UpdateAuthUser(ctx context.Context, id int64, input AuthUserMutation) (AuthUserRecord, error)
}

func (record AuthUserRecord) permissions() []string {
	switch strings.ToLower(strings.TrimSpace(record.Role)) {
	case RoleAdmin:
		return []string{RoleAdmin, PermissionStoreRead, PermissionStoreWrite, PermissionUserManage}
	case RoleEditor:
		return []string{RoleEditor, PermissionStoreRead, PermissionStoreWrite}
	default:
		return []string{RoleViewer, PermissionStoreRead}
	}
}

func (record AuthUserRecord) applyToResponse(user AuthUserResponse) AuthUserResponse {
	user.Email = strings.TrimSpace(record.Email)
	user.Username = firstNonEmpty(record.Username, user.Username, user.Email)
	user.DisplayName = firstNonEmpty(record.DisplayName, user.DisplayName, user.Username, user.Email)
	user.FeishuUserID = firstNonEmpty(record.FeishuUserID, user.FeishuUserID)
	user.Phone = firstNonEmpty(record.Phone, user.Phone)
	user.Role = firstNonEmpty(record.Role, "viewer")
	return user
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RoleAdmin:
		return RoleAdmin
	case RoleEditor:
		return RoleEditor
	default:
		return RoleViewer
	}
}

func scanAuthUser(scanner interface {
	Scan(dest ...any) error
}) (AuthUserRecord, error) {
	var record AuthUserRecord
	var lastLogin sql.NullTime
	err := scanner.Scan(
		&record.ID,
		&record.Email,
		&record.Username,
		&record.DisplayName,
		&record.FeishuUserID,
		&record.Phone,
		&record.Role,
		&record.Enabled,
		&lastLogin,
	)
	if err != nil {
		return AuthUserRecord{}, err
	}
	if lastLogin.Valid {
		record.LastLoginAt = &lastLogin.Time
	}
	return record, nil
}

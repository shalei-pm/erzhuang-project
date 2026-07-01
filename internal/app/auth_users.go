package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const defaultAdminEmail = "shalei@soyoung.com"

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

type AuthUserStore interface {
	GetAuthUserByEmail(ctx context.Context, email string) (AuthUserRecord, error)
	UpdateAuthUserProfile(ctx context.Context, patch AuthUserPatch) (AuthUserRecord, error)
}

func (record AuthUserRecord) permissions() []string {
	if strings.EqualFold(record.Role, "admin") {
		return []string{"admin"}
	}
	return []string{record.Role}
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

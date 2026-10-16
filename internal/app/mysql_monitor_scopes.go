package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

type monitorScopeQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type monitorScopeSnapshot struct {
	Scopes []AuthUserResourceScope
	Tokens []string
}

func monitorScopeRevision(tokens []string) string {
	ordered := append([]string{}, tokens...)
	sort.Strings(ordered)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(ordered, "\n"))))
}

func memoryScopeRevision(scopes []AuthUserResourceScope) string {
	tokens := []string{}
	for _, scope := range scopes {
		tokens = append(tokens, fmt.Sprintf("%s:%d:%s", ResourceTypeCRMInstitution, scope.StoreID, scope.ExternalOrgID))
	}
	return monitorScopeRevision(tokens)
}

func userMonitorScopeRevision(tokens []string, role string, enabled bool) string {
	values := append([]string{}, tokens...)
	values = append(values, fmt.Sprintf("user:%s:%t", normalizeRole(role), enabled))
	return monitorScopeRevision(values)
}

func canonicalInstitutionID(key string) (int64, bool) {
	id, err := strconv.ParseInt(key, 10, 64)
	return id, err == nil && id > 0 && id <= 9007199254740991 && strconv.FormatInt(id, 10) == key
}

func listCRMMonitorInstitutions(ctx context.Context, q monitorScopeQueryer, ids []int64, candidatesOnly bool) ([]AuthUserResourceScope, error) {
	query := "select t.id, t.name, t.city_id, t.status from tb_crm_admin_tenant t where t.id > 0"
	args := []any{}
	if candidatesOnly {
		query += " and " + resourceview.CRMInstitutionWithEdgeSQL
	}
	if ids != nil {
		if len(ids) == 0 {
			return []AuthUserResourceScope{}, nil
		}
		query += " and t.id in (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"
		for _, id := range ids {
			args = append(args, id)
		}
	}
	query += " order by t.city_id, t.name, t.id"
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopes := []AuthUserResourceScope{}
	for rows.Next() {
		var id int64
		var name sql.NullString
		var city, status sql.NullInt64
		if err := rows.Scan(&id, &name, &city, &status); err != nil {
			return nil, err
		}
		if _, valid := canonicalInstitutionID(strconv.FormatInt(id, 10)); !valid {
			return nil, fmt.Errorf("CRM 机构 ID %d 超出支持范围", id)
		}
		scopes = append(scopes, AuthUserResourceScope{StoreID: id, Name: firstNonEmpty(strings.TrimSpace(name.String), fmt.Sprintf("机构 %d", id)), City: resourceview.CityName(city.Int64), ExternalOrgID: strconv.FormatInt(id, 10), Unavailable: status.Int64 != 1})
	}
	return scopes, rows.Err()
}

// Read saved external identities, never infer a CRM ID from a legacy internal ID.
func readCRMMonitorScopes(ctx context.Context, q monitorScopeQueryer, userIDs []int64, enrich bool) (map[int64]monitorScopeSnapshot, error) {
	result := map[int64]monitorScopeSnapshot{}
	if len(userIDs) == 0 {
		return result, nil
	}
	args := []any{ResourceTypeStore, ResourceTypeCRMInstitution, ScopeMonitorView}
	for _, id := range userIDs {
		args = append(args, id)
		result[id] = monitorScopeSnapshot{Scopes: []AuthUserResourceScope{}}
	}
	locking := ""
	if _, ok := q.(*sql.Tx); ok {
		locking = " for update"
	}
	rows, err := q.QueryContext(ctx, `select user_id, resource_type, resource_id, external_key
		from tb_user_resource_scopes where resource_type in (?, ?) and scope = ? and user_id in (`+strings.TrimSuffix(strings.Repeat("?,", len(userIDs)), ",")+`) order by user_id, id`+locking, args...)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	seen := map[int64]map[int64]bool{}
	kinds := map[int64]string{}
	for rows.Next() {
		var userID, resourceID int64
		var kind, key string
		if err := rows.Scan(&userID, &kind, &resourceID, &key); err != nil {
			rows.Close()
			return nil, err
		}
		id, valid := canonicalInstitutionID(key)
		if !valid || (kind != ResourceTypeStore && kind != ResourceTypeCRMInstitution) || (kind == ResourceTypeCRMInstitution && resourceID != id) {
			rows.Close()
			return nil, fmt.Errorf("用户 %d 的历史机构授权标识异常，请先核对权限记录", userID)
		}
		if kinds[userID] != "" && kinds[userID] != kind {
			rows.Close()
			return nil, fmt.Errorf("用户 %d 的机构权限存在混合身份，请先核对权限记录", userID)
		}
		kinds[userID] = kind
		snapshot := result[userID]
		snapshot.Tokens = append(snapshot.Tokens, fmt.Sprintf("%s:%d:%s", kind, resourceID, key))
		result[userID] = snapshot
		if seen[userID] == nil {
			seen[userID] = map[int64]bool{}
		}
		if seen[userID][id] {
			continue
		}
		seen[userID][id] = true
		ids = append(ids, id)
		snapshot = result[userID]
		snapshot.Scopes = append(snapshot.Scopes, AuthUserResourceScope{StoreID: id, ExternalOrgID: key, Name: fmt.Sprintf("机构 %d", id), Unavailable: true})
		result[userID] = snapshot
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if !enrich || len(ids) == 0 {
		return result, nil
	}
	metadata, err := listCRMMonitorInstitutions(ctx, q, uniquePositiveInt64s(ids), false)
	if err != nil {
		return nil, err
	}
	byID := map[int64]AuthUserResourceScope{}
	for _, scope := range metadata {
		byID[scope.StoreID] = scope
	}
	for userID, snapshot := range result {
		scopes := snapshot.Scopes
		for i, scope := range scopes {
			if label, ok := byID[scope.StoreID]; ok {
				scopes[i] = label
			}
		}
		sort.Slice(scopes, func(i, j int) bool {
			if scopes[i].City != scopes[j].City {
				return scopes[i].City < scopes[j].City
			}
			if scopes[i].Name != scopes[j].Name {
				return scopes[i].Name < scopes[j].Name
			}
			return scopes[i].StoreID < scopes[j].StoreID
		})
		snapshot.Scopes = scopes
		result[userID] = snapshot
	}
	return result, nil
}

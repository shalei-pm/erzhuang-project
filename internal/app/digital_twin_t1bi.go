package app

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/digitaltwin/t1bi"
)

func (h *Handler) digitalTwinT1BIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	externalOrgID := strings.TrimSpace(r.PathValue("externalOrgId"))
	if !validExternalOrgID(externalOrgID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_tenant_id", "error": "机构参数无效"})
		return
	}
	if !h.digitalTwinCanViewStore(w, r, externalOrgID) {
		return
	}
	if h.digitalTwinT1BIService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "digital_twin_t1_bi_unavailable", "error": "T+1 运营数据暂未接入，请稍后重试"})
		return
	}
	identity, err := h.currentAuthIdentity(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	tenantID, _ := strconv.ParseInt(externalOrgID, 10, 64)
	result, err := h.digitalTwinT1BIService.Get(r.Context(), t1bi.Request{
		TenantID: tenantID,
		User: t1bi.UserBaseInfo{
			UserName:   firstNonEmpty(identity.record.Username, identity.user.Username),
			UserNameCN: firstNonEmpty(identity.record.DisplayName, identity.user.DisplayName),
			Mail:       firstNonEmpty(identity.record.Email, identity.user.Email),
		},
	})
	if errors.Is(err, t1bi.ErrInvalidTenant) || errors.Is(err, t1bi.ErrInvalidDate) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_t1_bi_query", "error": "T+1 数据查询参数无效"})
		return
	}
	if err != nil {
		log.Printf("digital twin t+1 bi unavailable: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "digital_twin_t1_bi_unavailable", "error": "T+1 运营数据暂时获取失败，已保留上次数据"})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

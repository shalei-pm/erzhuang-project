package t1bi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.sy.soyoung.com/go/hprose/rpc"
)

const DefaultServer = "tcp://nlb-ad2s30sjzo92sisvei.cn-beijing.nlb.aliyuncsslb.com:8081"

type dataAPIService struct {
	DataAPI func(string) (string, error) `name:"dataApi"`
}

type ProviderClient struct {
	transport *rpc.TCPClient
	service   *dataAPIService
	server    string
}

type requestPayload struct {
	MethodGroup  string       `json:"method_group"`
	MethodName   string       `json:"method_name"`
	PlatformType int          `json:"platform_type"`
	PlayLoad     payload      `json:"play_load"`
	UserBaseInfo UserBaseInfo `json:"user_base_info"`
}

type payload struct {
	TenantID int64  `json:"tenant_id"`
	BeginDay string `json:"begin_day"`
	EndDay   string `json:"end_day"`
}

type response struct {
	Code int          `json:"code"`
	Msg  string       `json:"msg"`
	Data responseData `json:"data"`
}

type responseData struct {
	Rows []responseRow `json:"data"`
}

type responseRow struct {
	Day                      string          `json:"st_day"`
	VisitUserCount           json.RawMessage `json:"visit_user_count"`
	VisitFrontDesk           json.RawMessage `json:"visit_front_desk"`
	VisitNoConsult           json.RawMessage `json:"visit_no_consult"`
	VisitNeedConsult         json.RawMessage `json:"visit_need_consult"`
	AvgInStoreMinutes        json.RawMessage `json:"avg_in_store_minutes"`
	AvgWaitMinutes           json.RawMessage `json:"avg_wait_minutes"`
	UpgradeRate              json.RawMessage `json:"upgrade_rate"`
	AvgWriteoffIncome        json.RawMessage `json:"avg_writeoff_income"`
	AvgWriteoffServicePoints json.RawMessage `json:"avg_writeoff_service_points"`
}

func NewProvider(server string, timeout time.Duration) (*ProviderClient, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		server = DefaultServer
	}
	if timeout <= 0 {
		return nil, errors.New("t+1 bi timeout must be positive")
	}
	u, err := url.Parse(server)
	if err != nil || (u.Scheme != "tcp" && u.Scheme != "tcp4" && u.Scheme != "tcp6") {
		return nil, fmt.Errorf("t+1 bi server must be a tcp://host:port URI: %q", server)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("t+1 bi server must not contain a path: %q", server)
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || host == "" {
		return nil, fmt.Errorf("t+1 bi server must contain host and port: %q", server)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber <= 0 || portNumber > 65535 {
		return nil, fmt.Errorf("t+1 bi server has invalid port: %q", server)
	}
	transport := rpc.NewTCPClient(server)
	transport.SetTimeout(timeout)
	var remote *dataAPIService
	transport.UseService(&remote)
	return &ProviderClient{transport: transport, service: remote, server: server}, nil
}

func (c *ProviderClient) Close() {
	if c != nil && c.transport != nil {
		c.transport.Close()
	}
}

func (c *ProviderClient) GetDailyMetrics(_ context.Context, request Request) ([]DailyMetric, error) {
	if c == nil || c.service == nil || c.service.DataAPI == nil {
		return nil, errors.New("t+1 bi client is not initialized")
	}
	if request.TenantID <= 0 {
		return nil, ErrInvalidTenant
	}
	begin, end := request.BeginDay, request.EndDay
	beginDate, beginErr := time.Parse(DateLayout, begin)
	endDate, endErr := time.Parse(DateLayout, end)
	if beginErr != nil || endErr != nil || beginDate.Format(DateLayout) != begin || endDate.Format(DateLayout) != end || beginDate.After(endDate) {
		return nil, ErrInvalidDate
	}
	param, err := json.Marshal(requestPayload{
		MethodGroup:  "chain_dashboard",
		MethodName:   "sha_all_customer_daily_metrics",
		PlatformType: 4,
		PlayLoad:     payload{TenantID: request.TenantID, BeginDay: begin, EndDay: end},
		UserBaseInfo: request.User,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal t+1 bi request: %w", err)
	}
	raw, err := c.service.DataAPI(string(param))
	if err != nil {
		return nil, fmt.Errorf("invoke dataApi: %w", err)
	}
	return decodeRows(raw)
}

func decodeRows(raw string) ([]DailyMetric, error) {
	var decoded response
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("decode dataApi response: %w", err)
	}
	if decoded.Code != 200 {
		return nil, fmt.Errorf("dataApi returned code=%d msg=%q", decoded.Code, decoded.Msg)
	}
	rows := make([]DailyMetric, 0, len(decoded.Data.Rows))
	for _, row := range decoded.Data.Rows {
		rows = append(rows, DailyMetric{
			Day:                      row.Day,
			VisitUserCount:           nullableNumber(row.VisitUserCount),
			VisitFrontDesk:           nullableNumber(row.VisitFrontDesk),
			VisitNoConsult:           nullableNumber(row.VisitNoConsult),
			VisitNeedConsult:         nullableNumber(row.VisitNeedConsult),
			AvgInStoreMinutes:        nullableNumber(row.AvgInStoreMinutes),
			AvgWaitMinutes:           nullableNumber(row.AvgWaitMinutes),
			UpgradeRate:              nullableNumber(row.UpgradeRate),
			AvgWriteoffIncome:        nullableNumber(row.AvgWriteoffIncome),
			AvgWriteoffServicePoints: nullableNumber(row.AvgWriteoffServicePoints),
		})
	}
	return rows, nil
}

func nullableNumber(raw json.RawMessage) *float64 {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return nil
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return nil
		}
		value = strings.TrimSpace(text)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 {
		return nil
	}
	return &number
}

package t1bi

import (
	"context"
	"time"
)

const DateLayout = "2006-01-02"

type UserBaseInfo struct {
	UserName   string `json:"user_name"`
	UserNameCN string `json:"user_name_cn"`
	Mail       string `json:"mail"`
}

type Request struct {
	TenantID int64
	Days     int
	BeginDay string
	EndDay   string
	User     UserBaseInfo
}

type DailyMetric struct {
	Day                      string
	VisitUserCount           *float64
	VisitNoConsult           *float64
	VisitNeedConsult         *float64
	AvgInStoreMinutes        *float64
	AvgWaitMinutes           *float64
	UpgradeRate              *float64
	AvgWriteoffIncome        *float64
	AvgWriteoffServicePoints *float64
}

type Trend struct {
	Date             string   `json:"date"`
	VisitAll         *float64 `json:"visit_all"`
	VisitNoConsult   *float64 `json:"visit_no_consult"`
	VisitNeedConsult *float64 `json:"visit_need_consult"`
	StayAll          *float64 `json:"stay_all"`
	WaitAll          *float64 `json:"wait_all"`
	UpgradeAll       *float64 `json:"upgrade_all"`
	RedemptionAll    *float64 `json:"redemption_all"`
	ServicePointAll  *float64 `json:"service_point_all"`
}

type Dashboard struct {
	TenantID  int64     `json:"tenant_id"`
	BeginDay  string    `json:"begin_day"`
	EndDay    string    `json:"end_day"`
	FetchedAt time.Time `json:"fetched_at"`
	Trends    []Trend   `json:"trends"`
}

type Provider interface {
	GetDailyMetrics(context.Context, Request) ([]DailyMetric, error)
}

package t1bi

import (
	"context"
	"errors"
	"testing"
	"time"
)

type providerStub struct {
	rows []DailyMetric
	err  error
	req  Request
}

func (p *providerStub) GetDailyMetrics(_ context.Context, request Request) ([]DailyMetric, error) {
	p.req = request
	return p.rows, p.err
}

func TestServiceMapsDailyMetricsAndSortsDates(t *testing.T) {
	provider := &providerStub{rows: []DailyMetric{
		{Day: "2026-09-14", VisitUserCount: number(20), VisitFrontDesk: number(3), VisitNoConsult: number(12), VisitNeedConsult: number(8), AvgInStoreMinutes: number(48), AvgWaitMinutes: number(9), UpgradeRate: number(12.5), AvgWriteoffIncome: number(680.5), AvgWriteoffServicePoints: number(2.4)},
		{Day: "2026-09-13", VisitUserCount: number(18)},
	}}
	service := NewService(provider)
	service.now = func() time.Time { return time.Date(2026, 9, 16, 9, 0, 0, 0, businessTimeZone) }
	result, err := service.Get(context.Background(), Request{TenantID: 10001, User: UserBaseInfo{Mail: "tester@soyoung.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if provider.req.TenantID != 10001 || provider.req.Days != 30 || provider.req.BeginDay != "2026-08-17" || provider.req.EndDay != "2026-09-15" || provider.req.User.Mail != "tester@soyoung.com" {
		t.Fatalf("request = %#v", provider.req)
	}
	if result.BeginDay != "2026-08-17" || result.EndDay != "2026-09-15" || len(result.Trends) != 30 {
		t.Fatalf("result = %#v", result)
	}
	if result.Trends[0].Date != "2026-08-17" || result.Trends[27].Date != "2026-09-13" || result.Trends[28].RedemptionAll == nil || *result.Trends[28].RedemptionAll != 680.5 || result.Trends[28].VisitFrontDesk == nil || *result.Trends[28].VisitFrontDesk != 3 || result.Trends[28].VisitNoConsult == nil || *result.Trends[28].VisitNoConsult != 12 || result.Trends[28].VisitNeedConsult == nil || *result.Trends[28].VisitNeedConsult != 8 || result.Trends[29].Date != "2026-09-15" || result.Trends[29].VisitAll != nil {
		t.Fatalf("trends = %#v", result.Trends)
	}
}

func TestServiceKeepsProviderErrorsAsUnavailable(t *testing.T) {
	service := NewService(&providerStub{err: errors.New("dataApi down")})
	_, err := service.Get(context.Background(), Request{TenantID: 10001})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestServiceRejectsDuplicateOrOutOfRangeDays(t *testing.T) {
	for _, rows := range [][]DailyMetric{
		{{Day: "2026-09-14"}, {Day: "2026-09-14"}},
		{{Day: "2026-08-16"}},
	} {
		service := NewService(&providerStub{rows: rows})
		service.now = func() time.Time { return time.Date(2026, 9, 16, 9, 0, 0, 0, businessTimeZone) }
		if _, err := service.Get(context.Background(), Request{TenantID: 10001}); !errors.Is(err, ErrInvalidDate) {
			t.Fatalf("rows %#v: expected invalid date, got %v", rows, err)
		}
	}
}

func TestRecentCalendarDays(t *testing.T) {
	day := time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)
	begin, end, err := RecentCalendarDays(day, 30)
	if err != nil || begin != "2026-08-17" || end != "2026-09-15" {
		t.Fatalf("range = %q, %q, %v", begin, end, err)
	}
	if _, _, err := RecentCalendarDays(day, 0); err == nil {
		t.Fatal("expected invalid days error")
	}
}

func number(value float64) *float64 { return &value }

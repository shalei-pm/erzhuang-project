package t1bi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrUnavailable   = errors.New("t+1 bi provider unavailable")
	ErrInvalidDate   = errors.New("t+1 bi date range is invalid")
	ErrInvalidTenant = errors.New("t+1 bi tenant id must be positive")
)

const defaultDays = 30

var businessTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

type Service struct {
	provider Provider
	days     int
	now      func() time.Time
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider, days: defaultDays, now: time.Now}
}

func (s *Service) Get(ctx context.Context, request Request) (Dashboard, error) {
	if request.TenantID <= 0 {
		return Dashboard{}, ErrInvalidTenant
	}
	if s == nil || s.provider == nil {
		return Dashboard{}, ErrUnavailable
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	days := request.Days
	if days == 0 {
		days = s.days
	}
	begin, end, err := RecentCalendarDays(now, days)
	if err != nil {
		return Dashboard{}, err
	}
	request.User = normalizeUser(request.User)
	rows, err := s.provider.GetDailyMetrics(ctx, Request{
		TenantID: request.TenantID,
		Days:     days,
		BeginDay: begin,
		EndDay:   end,
		User:     request.User,
	})
	if err != nil {
		return Dashboard{}, errors.Join(ErrUnavailable, err)
	}
	trends := make([]Trend, 0, len(rows))
	byDay := make(map[string]Trend, len(rows))
	for _, row := range rows {
		day, err := time.Parse(DateLayout, row.Day)
		if err != nil || day.Format(DateLayout) != row.Day || row.Day < begin || row.Day > end {
			return Dashboard{}, fmt.Errorf("%w: provider returned day %q", ErrInvalidDate, row.Day)
		}
		if _, exists := byDay[row.Day]; exists {
			return Dashboard{}, fmt.Errorf("%w: provider returned duplicate day %q", ErrInvalidDate, row.Day)
		}
		byDay[row.Day] = Trend{
			Date:             row.Day,
			VisitAll:         row.VisitUserCount,
			VisitFrontDesk:   row.VisitFrontDesk,
			VisitNoConsult:   row.VisitNoConsult,
			VisitNeedConsult: row.VisitNeedConsult,
			StayAll:          row.AvgInStoreMinutes,
			WaitAll:          row.AvgWaitMinutes,
			UpgradeAll:       row.UpgradeRate,
			RedemptionAll:    row.AvgWriteoffIncome,
			ServicePointAll:  row.AvgWriteoffServicePoints,
		}
	}
	firstDay, _ := time.ParseInLocation(DateLayout, begin, businessTimeZone)
	for day := firstDay; len(trends) < days; day = day.AddDate(0, 0, 1) {
		date := day.Format(DateLayout)
		trend, exists := byDay[date]
		if !exists {
			trend = Trend{Date: date}
		}
		trends = append(trends, trend)
	}
	return Dashboard{TenantID: request.TenantID, BeginDay: begin, EndDay: end, FetchedAt: now.UTC(), Trends: trends}, nil
}

func RecentCalendarDays(now time.Time, days int) (begin, end string, err error) {
	if days <= 0 || days > 366 {
		return "", "", fmt.Errorf("days must be between 1 and 366")
	}
	lastCompletedDay := now.In(businessTimeZone).AddDate(0, 0, -1)
	return lastCompletedDay.AddDate(0, 0, -(days - 1)).Format(DateLayout), lastCompletedDay.Format(DateLayout), nil
}

func normalizeUser(user UserBaseInfo) UserBaseInfo {
	user.UserName = strings.TrimSpace(user.UserName)
	user.UserNameCN = strings.TrimSpace(user.UserNameCN)
	user.Mail = strings.TrimSpace(user.Mail)
	return user
}

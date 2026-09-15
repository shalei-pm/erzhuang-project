package digitaltwin

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnavailable = errors.New("digital twin metrics provider unavailable")
	ErrInvalidDate = errors.New("digital twin date must use yyyy-mm-dd")
)

const defaultTimeout = 5 * time.Second

var businessTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

type Service struct {
	provider Provider
	timeout  time.Duration
	now      func() time.Time
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider, timeout: defaultTimeout, now: time.Now}
}

func (s *Service) Get(ctx context.Context, request Request) (Dashboard, error) {
	if request.TenantID <= 0 {
		return Dashboard{}, ErrInvalidTenant
	}
	now := time.Now()
	if s != nil && s.now != nil {
		now = s.now()
	}
	date, err := normalizeDate(request.Date, now)
	if err != nil {
		return Dashboard{}, err
	}
	if s == nil || s.provider == nil {
		return Dashboard{}, ErrUnavailable
	}

	callCtx := ctx
	if s.timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	request.Date = date

	var (
		overview Overview
		staff    DutyStaff
		traffic  TrafficFlow
		errs     [3]error
		wg       sync.WaitGroup
	)
	wg.Add(3)
	go func() { defer wg.Done(); overview, errs[0] = s.provider.GetOverview(callCtx, request) }()
	go func() { defer wg.Done(); staff, errs[1] = s.provider.GetDutyStaff(callCtx, request) }()
	go func() { defer wg.Done(); traffic, errs[2] = s.provider.GetTrafficFlow(callCtx, request) }()
	wg.Wait()
	for _, callErr := range errs {
		if callErr != nil {
			return Dashboard{}, errors.Join(ErrUnavailable, callErr)
		}
	}

	return Dashboard{TenantID: request.TenantID, Date: date, FetchedAt: now.UTC(), Overview: overview, DutyStaff: staff, Traffic: traffic}, nil
}

func normalizeDate(value string, now time.Time) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return now.In(businessTimeZone).Format("2006-01-02"), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return "", ErrInvalidDate
	}
	return value, nil
}

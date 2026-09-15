package digitaltwin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProvider struct {
	overview Overview
	staff    DutyStaff
	traffic  TrafficFlow
	err      error
	calls    atomic.Int32
}

func (p *fakeProvider) GetOverview(context.Context, Request) (Overview, error) {
	p.calls.Add(1)
	return p.overview, p.err
}

func (p *fakeProvider) GetDutyStaff(context.Context, Request) (DutyStaff, error) {
	p.calls.Add(1)
	return p.staff, p.err
}

func (p *fakeProvider) GetTrafficFlow(context.Context, Request) (TrafficFlow, error) {
	p.calls.Add(1)
	return p.traffic, p.err
}

func TestServiceCallsAllRPCsAndPreservesZeroValuesAsBusinessData(t *testing.T) {
	provider := &fakeProvider{overview: Overview{ExpectedArrival: 12}, staff: DutyStaff{Nurses: 2}}
	service := NewService(provider)
	service.now = func() time.Time { return time.Date(2026, 9, 14, 8, 30, 0, 0, time.FixedZone("CST", 8*60*60)) }

	result, err := service.Get(context.Background(), Request{TenantID: 10001, Date: "2026-09-14"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if provider.calls.Load() != 3 {
		t.Fatalf("provider calls = %d, want 3", provider.calls.Load())
	}
	if result.Overview.ExpectedArrival != 12 || result.DutyStaff.Nurses != 2 || result.Traffic.Waiting != 0 {
		t.Fatalf("result = %#v", result)
	}
	if got := result.FetchedAt.Format(time.RFC3339); got != "2026-09-14T00:30:00Z" {
		t.Fatalf("FetchedAt = %s", got)
	}
}

func TestServiceReturnsUnavailableWhenAnyRPCFailsInsteadOfReturningPartialZeros(t *testing.T) {
	provider := &fakeProvider{overview: Overview{ExpectedArrival: 12}, err: errors.New("rpc down")}
	service := NewService(provider)

	result, err := service.Get(context.Background(), Request{TenantID: 10001})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
	if result != (Dashboard{}) {
		t.Fatalf("partial dashboard = %#v, want empty result", result)
	}
}

func TestServiceValidatesDateAndTenant(t *testing.T) {
	service := NewService(&fakeProvider{})
	for _, test := range []struct {
		name string
		in   Request
		want error
	}{
		{name: "tenant", in: Request{TenantID: 0}, want: ErrInvalidTenant},
		{name: "date", in: Request{TenantID: 10001, Date: "2026-2-1"}, want: ErrInvalidDate},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Get(context.Background(), test.in)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

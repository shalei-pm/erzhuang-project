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
	dates    chan string
}

func (p *fakeProvider) record(request Request) {
	if p.dates != nil {
		p.dates <- request.Date
	}
}

func (p *fakeProvider) GetOverview(_ context.Context, request Request) (Overview, error) {
	p.calls.Add(1)
	p.record(request)
	return p.overview, p.err
}

func (p *fakeProvider) GetDutyStaff(_ context.Context, request Request) (DutyStaff, error) {
	p.calls.Add(1)
	p.record(request)
	return p.staff, p.err
}

func (p *fakeProvider) GetTrafficFlow(_ context.Context, request Request) (TrafficFlow, error) {
	p.calls.Add(1)
	p.record(request)
	return p.traffic, p.err
}

func TestServiceDefaultsEmptyDateToShanghaiBusinessDate(t *testing.T) {
	provider := &fakeProvider{dates: make(chan string, 3)}
	service := NewService(provider)
	service.now = func() time.Time { return time.Date(2026, 9, 14, 16, 30, 0, 0, time.UTC) }

	result, err := service.Get(context.Background(), Request{TenantID: 10001})
	if err != nil {
		t.Fatal(err)
	}
	if result.Date != "2026-09-15" {
		t.Fatalf("Date = %q", result.Date)
	}
	for range 3 {
		if got := <-provider.dates; got != "2026-09-15" {
			t.Fatalf("provider date = %q", got)
		}
	}
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

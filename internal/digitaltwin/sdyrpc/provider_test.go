package sdyrpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/digitaltwin"
	"github.com/shalei-pm/erzhuang-project/internal/digitaltwin/primecrm"
)

func TestProviderMapsRequestsAndResponses(t *testing.T) {
	wantRequest := &primecrm.QuickMedicalBeautyDashboardRequest{TenantId: 10001, Date: "2026-09-14"}
	provider := newProvider(
		func(_ context.Context, request *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyOverviewResponse, error) {
			assertRequest(t, request, wantRequest)
			return &primecrm.QuickMedicalBeautyOverviewResponse{ExpectedArrivalCount: 11, ArrivedCount: 9, NoConsultCount: 3, NeedConsultCount: 6, NonQuickMedicalBeautyCount: 2}, nil
		},
		func(_ context.Context, request *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyDutyStaffResponse, error) {
			assertRequest(t, request, wantRequest)
			return &primecrm.QuickMedicalBeautyDutyStaffResponse{ConsultantCount: 4, NurseCount: 5, DoctorCount: 6}, nil
		},
		func(_ context.Context, request *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyTrafficFlowResponse, error) {
			assertRequest(t, request, wantRequest)
			return &primecrm.QuickMedicalBeautyTrafficFlowResponse{ReceptionCurrentCount: 1, ConsultationCurrentCount: 2, ConsultationServedCount: 3, WaitingCount: 4, WaitingNoConsultCount: 5, WaitingNeedConsultCount: 6, WaitingNonQuickMedicalBeautyCount: 7, TreatmentCurrentCount: 8, TreatmentServedCount: 9, TreatmentServedNoConsultCount: 10, TreatmentServedNeedConsultCount: 11, TreatmentServedNonQuickMedicalBeautyCount: 12, PostoperativeCareCount: 13}, nil
		},
	)
	request := digitaltwin.Request{TenantID: 10001, Date: "2026-09-14"}

	overview, err := provider.GetOverview(context.Background(), request)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview != (digitaltwin.Overview{ExpectedArrival: 11, Arrived: 9, NoConsult: 3, NeedConsult: 6, NonQuick: 2}) {
		t.Fatalf("overview = %#v", overview)
	}

	staff, err := provider.GetDutyStaff(context.Background(), request)
	if err != nil {
		t.Fatalf("GetDutyStaff() error = %v", err)
	}
	if staff != (digitaltwin.DutyStaff{Consultants: 4, Nurses: 5, Doctors: 6}) {
		t.Fatalf("staff = %#v", staff)
	}

	traffic, err := provider.GetTrafficFlow(context.Background(), request)
	if err != nil {
		t.Fatalf("GetTrafficFlow() error = %v", err)
	}
	if traffic != (digitaltwin.TrafficFlow{ReceptionCurrent: 1, ConsultationCurrent: 2, ConsultationServed: 3, Waiting: 4, WaitingNoConsult: 5, WaitingNeedConsult: 6, WaitingNonQuick: 7, TreatmentCurrent: 8, TreatmentServed: 9, TreatmentServedNoConsult: 10, TreatmentServedNeedConsult: 11, TreatmentServedNonQuick: 12, PostoperativeCare: 13}) {
		t.Fatalf("traffic = %#v", traffic)
	}
}

func TestProviderReturnsContextualErrorForRPCFailure(t *testing.T) {
	provider := newProvider(
		func(context.Context, *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyOverviewResponse, error) {
			return nil, errors.New("rpc down")
		},
		nil,
		nil,
	)

	_, err := provider.GetOverview(context.Background(), digitaltwin.Request{TenantID: 10001, Date: "2026-09-14"})
	if err == nil || !strings.Contains(err.Error(), "SdyService.GetQuickMedicalBeautyOverview tenant_id=10001 date=2026-09-14") {
		t.Fatalf("error = %v", err)
	}
}

func TestProviderRecoversRPCClientPanic(t *testing.T) {
	provider := newProvider(
		func(context.Context, *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyOverviewResponse, error) {
			panic("registry unavailable")
		},
		nil,
		nil,
	)

	_, err := provider.GetOverview(context.Background(), digitaltwin.Request{TenantID: 10001})
	if err == nil || !strings.Contains(err.Error(), "registry unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func assertRequest(t *testing.T, got, want *primecrm.QuickMedicalBeautyDashboardRequest) {
	t.Helper()
	if got.GetTenantId() != want.GetTenantId() || got.GetDate() != want.GetDate() {
		t.Fatalf("request = %#v, want %#v", got, want)
	}
}

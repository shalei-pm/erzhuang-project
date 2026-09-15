package sdyrpc

import (
	"context"
	"errors"
	"fmt"

	"gitlab.sy.soyoung.com/go/library/service/dubbo"

	"github.com/shalei-pm/erzhuang-project/internal/digitaltwin"
	"github.com/shalei-pm/erzhuang-project/internal/digitaltwin/primecrm"
)

type overviewCall func(context.Context, *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyOverviewResponse, error)
type dutyStaffCall func(context.Context, *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyDutyStaffResponse, error)
type trafficFlowCall func(context.Context, *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyTrafficFlowResponse, error)

// Provider adapts the generated Triple client to the dashboard's small domain contract.
type Provider struct {
	overview    overviewCall
	dutyStaff   dutyStaffCall
	trafficFlow trafficFlowCall
}

func newProvider(overview overviewCall, dutyStaff dutyStaffCall, trafficFlow trafficFlowCall) *Provider {
	return &Provider{overview: overview, dutyStaff: dutyStaff, trafficFlow: trafficFlow}
}

// NewProvider obtains the company's service-discovery-backed client. The base
// library chooses ZooKeeper endpoints from APP_RUN_ENV, so this package does
// not contain environment-specific provider addresses.
func NewProvider() (provider *Provider, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("initialize SdyService client: %v", recovered)
			provider = nil
		}
	}()

	client := dubbo.GetClient[primecrm.SdyServiceClientImpl]()
	if client == nil {
		return nil, errors.New("initialize SdyService client: company dubbo client is nil")
	}
	return newProvider(client.GetQuickMedicalBeautyOverview, client.GetQuickMedicalBeautyDutyStaff, client.GetQuickMedicalBeautyTrafficFlow), nil
}

func (p *Provider) GetOverview(ctx context.Context, request digitaltwin.Request) (result digitaltwin.Overview, err error) {
	const method = "GetQuickMedicalBeautyOverview"
	defer recoverRPC(method, request, &err)
	if p == nil || p.overview == nil {
		return result, rpcError(method, request, errors.New("client method unavailable"))
	}
	response, callErr := p.overview(ctx, dashboardRequest(request))
	if callErr != nil {
		return result, rpcError(method, request, callErr)
	}
	if response == nil {
		return result, rpcError(method, request, errors.New("empty response"))
	}
	return digitaltwin.Overview{
		ExpectedArrival: response.GetExpectedArrivalCount(),
		Arrived:         response.GetArrivedCount(),
		NoConsult:       response.GetNoConsultCount(),
		NeedConsult:     response.GetNeedConsultCount(),
		NonQuick:        response.GetNonQuickMedicalBeautyCount(),
	}, nil
}

func (p *Provider) GetDutyStaff(ctx context.Context, request digitaltwin.Request) (result digitaltwin.DutyStaff, err error) {
	const method = "GetQuickMedicalBeautyDutyStaff"
	defer recoverRPC(method, request, &err)
	if p == nil || p.dutyStaff == nil {
		return result, rpcError(method, request, errors.New("client method unavailable"))
	}
	response, callErr := p.dutyStaff(ctx, dashboardRequest(request))
	if callErr != nil {
		return result, rpcError(method, request, callErr)
	}
	if response == nil {
		return result, rpcError(method, request, errors.New("empty response"))
	}
	return digitaltwin.DutyStaff{
		Consultants: response.GetConsultantCount(),
		Nurses:      response.GetNurseCount(),
		Doctors:     response.GetDoctorCount(),
	}, nil
}

func (p *Provider) GetTrafficFlow(ctx context.Context, request digitaltwin.Request) (result digitaltwin.TrafficFlow, err error) {
	const method = "GetQuickMedicalBeautyTrafficFlow"
	defer recoverRPC(method, request, &err)
	if p == nil || p.trafficFlow == nil {
		return result, rpcError(method, request, errors.New("client method unavailable"))
	}
	response, callErr := p.trafficFlow(ctx, dashboardRequest(request))
	if callErr != nil {
		return result, rpcError(method, request, callErr)
	}
	if response == nil {
		return result, rpcError(method, request, errors.New("empty response"))
	}
	return digitaltwin.TrafficFlow{
		ReceptionCurrent:           response.GetReceptionCurrentCount(),
		ConsultationCurrent:        response.GetConsultationCurrentCount(),
		ConsultationServed:         response.GetConsultationServedCount(),
		Waiting:                    response.GetWaitingCount(),
		WaitingNoConsult:           response.GetWaitingNoConsultCount(),
		WaitingNeedConsult:         response.GetWaitingNeedConsultCount(),
		WaitingNonQuick:            response.GetWaitingNonQuickMedicalBeautyCount(),
		TreatmentCurrent:           response.GetTreatmentCurrentCount(),
		TreatmentServed:            response.GetTreatmentServedCount(),
		TreatmentServedNoConsult:   response.GetTreatmentServedNoConsultCount(),
		TreatmentServedNeedConsult: response.GetTreatmentServedNeedConsultCount(),
		TreatmentServedNonQuick:    response.GetTreatmentServedNonQuickMedicalBeautyCount(),
		PostoperativeCare:          response.GetPostoperativeCareCount(),
	}, nil
}

func dashboardRequest(request digitaltwin.Request) *primecrm.QuickMedicalBeautyDashboardRequest {
	return &primecrm.QuickMedicalBeautyDashboardRequest{TenantId: request.TenantID, Date: request.Date}
}

func rpcError(method string, request digitaltwin.Request, cause error) error {
	return fmt.Errorf("SdyService.%s tenant_id=%d date=%s: %w", method, request.TenantID, request.Date, cause)
}

func recoverRPC(method string, request digitaltwin.Request, target *error) {
	if recovered := recover(); recovered != nil {
		*target = rpcError(method, request, fmt.Errorf("client panic: %v", recovered))
	}
}

var _ digitaltwin.Provider = (*Provider)(nil)

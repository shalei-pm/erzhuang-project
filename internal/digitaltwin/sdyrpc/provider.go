package sdyrpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"dubbo.apache.org/dubbo-go/v3/config"
	_ "dubbo.apache.org/dubbo-go/v3/imports"

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
	prepare     func() error
}

func newProvider(overview overviewCall, dutyStaff dutyStaffCall, trafficFlow trafficFlowCall) *Provider {
	return &Provider{overview: overview, dutyStaff: dutyStaff, trafficFlow: trafficFlow}
}

// NewProvider creates a Triple client that follows the company's ZooKeeper
// environment convention without depending on the Linux-only qconf runtime.
func NewProvider() (provider *Provider, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("initialize SdyService client: %v", recovered)
			provider = nil
		}
	}()

	client := &primecrm.SdyServiceClientImpl{}
	provider = newProvider(
		func(ctx context.Context, request *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyOverviewResponse, error) {
			if client.GetQuickMedicalBeautyOverview == nil {
				return nil, errors.New("client method unavailable")
			}
			return client.GetQuickMedicalBeautyOverview(ctx, request)
		},
		func(ctx context.Context, request *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyDutyStaffResponse, error) {
			if client.GetQuickMedicalBeautyDutyStaff == nil {
				return nil, errors.New("client method unavailable")
			}
			return client.GetQuickMedicalBeautyDutyStaff(ctx, request)
		},
		func(ctx context.Context, request *primecrm.QuickMedicalBeautyDashboardRequest) (*primecrm.QuickMedicalBeautyTrafficFlowResponse, error) {
			if client.GetQuickMedicalBeautyTrafficFlow == nil {
				return nil, errors.New("client method unavailable")
			}
			return client.GetQuickMedicalBeautyTrafficFlow(ctx, request)
		},
	)
	provider.prepare = newClientInitializer(client)
	return provider, nil
}

func (p *Provider) GetOverview(ctx context.Context, request digitaltwin.Request) (result digitaltwin.Overview, err error) {
	const method = "GetQuickMedicalBeautyOverview"
	defer recoverRPC(method, request, &err)
	if prepareErr := p.prepareClient(); prepareErr != nil {
		return result, rpcError(method, request, prepareErr)
	}
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
	if prepareErr := p.prepareClient(); prepareErr != nil {
		return result, rpcError(method, request, prepareErr)
	}
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
	if prepareErr := p.prepareClient(); prepareErr != nil {
		return result, rpcError(method, request, prepareErr)
	}
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

func (p *Provider) prepareClient() error {
	if p == nil || p.prepare == nil {
		return nil
	}
	return p.prepare()
}

type rpcRuntimeConfig struct {
	environment string
	application string
	zkAddress   string
}

func newClientInitializer(client *primecrm.SdyServiceClientImpl) func() error {
	return newRetryingInitializer(func() error {
		return initializeClient(client)
	}, time.Now, 30*time.Second)
}

func newRetryingInitializer(initialize func() error, now func() time.Time, retryDelay time.Duration) func() error {
	var mu sync.Mutex
	var initialized bool
	var lastErr error
	var retryAfter time.Time

	return func() error {
		mu.Lock()
		defer mu.Unlock()

		if initialized {
			return nil
		}
		currentTime := now()
		if lastErr != nil && currentTime.Before(retryAfter) {
			return lastErr
		}
		if err := initialize(); err != nil {
			lastErr = err
			retryAfter = currentTime.Add(retryDelay)
			return err
		}
		initialized = true
		lastErr = nil
		return nil
	}
}

func initializeClient(client *primecrm.SdyServiceClientImpl) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("initialize SdyService Triple client: %v", recovered)
		}
	}()
	if client == nil {
		return errors.New("initialize SdyService Triple client: client is nil")
	}
	runtimeConfig, err := rpcConfigForEnvironment(os.Getenv("APP_RUN_ENV"), os.Getenv("GO_APP_RUN_ENV"), os.Getenv("APP_NAME"))
	if err != nil {
		return err
	}
	log.Printf("digital twin RPC client initialize rpc_env=%s application=%s", runtimeConfig.environment, runtimeConfig.application)

	config.SetConsumerService(client)
	reference := config.NewReferenceConfigBuilder().
		SetInterface("com.soyoung.primecrm.SdyService").
		SetProtocol("tri").
		Build()
	root := config.NewRootConfigBuilder().
		SetApplication(config.NewApplicationConfigBuilder().
			SetName(runtimeConfig.application).
			SetEnvironment(runtimeConfig.environment).
			Build()).
		AddRegistry("zk", config.NewRegistryConfigBuilder().
			SetProtocol("zookeeper").
			SetTimeout("3s").
			SetRegistryType("interface").
			SetAddress(runtimeConfig.zkAddress).
			Build()).
		SetMetadataReport(config.NewMetadataReportConfigBuilder().
			SetProtocol("zk").
			SetAddress(runtimeConfig.zkAddress).
			SetTimeout("3s").
			Build()).
		SetConsumer(config.NewConsumerConfigBuilder().
			AddReference("SdyServiceClientImpl", reference).
			Build())
	if err := root.Build().Init(); err != nil {
		log.Printf("digital twin RPC client initialization failed rpc_env=%s application=%s stage=registry error_type=%T", runtimeConfig.environment, runtimeConfig.application, err)
		return errors.New("initialize SdyService Triple client: registry initialization failed")
	}
	if client.GetQuickMedicalBeautyOverview == nil || client.GetQuickMedicalBeautyDutyStaff == nil || client.GetQuickMedicalBeautyTrafficFlow == nil {
		log.Printf("digital twin RPC client initialization failed rpc_env=%s application=%s stage=methods", runtimeConfig.environment, runtimeConfig.application)
		return errors.New("initialize SdyService Triple client: generated methods unavailable")
	}
	log.Printf("digital twin RPC client initialized rpc_env=%s application=%s", runtimeConfig.environment, runtimeConfig.application)
	return nil
}

func rpcConfigForEnvironment(primaryEnv, legacyEnv, appName string) (rpcRuntimeConfig, error) {
	environment := configuredRPCEnvironment(primaryEnv, legacyEnv)
	addresses := map[string]string{
		"local": "10.10.10.100:2181",
		"test":  "10.10.10.100:2181",
		"pre":   "172.16.16.60:2181",
		"prod":  "register-center1.sy.soyoung.com:2181,register-center2.sy.soyoung.com:2181,register-center3.sy.soyoung.com:2181,register-center4.sy.soyoung.com:2181,register-center5.sy.soyoung.com:2181",
	}
	zkAddress, ok := addresses[environment]
	if !ok {
		return rpcRuntimeConfig{}, fmt.Errorf("initialize SdyService Triple client: unsupported APP_RUN_ENV %q", environment)
	}
	appName = strings.TrimSpace(appName)
	if appName == "" {
		appName = "erzhuang-project"
	}
	return rpcRuntimeConfig{environment: environment, application: appName + "-serve", zkAddress: zkAddress}, nil
}

func configuredRPCEnvironment(primaryEnv, legacyEnv string) string {
	environment := strings.ToLower(strings.TrimSpace(primaryEnv))
	if environment == "" {
		environment = strings.ToLower(strings.TrimSpace(legacyEnv))
	}
	if environment == "" {
		return "local"
	}
	return environment
}

func dashboardRequest(request digitaltwin.Request) *primecrm.QuickMedicalBeautyDashboardRequest {
	return &primecrm.QuickMedicalBeautyDashboardRequest{TenantId: request.TenantID, Date: request.Date}
}

func rpcError(method string, request digitaltwin.Request, cause error) error {
	environment := configuredRPCEnvironment(os.Getenv("APP_RUN_ENV"), os.Getenv("GO_APP_RUN_ENV"))
	return fmt.Errorf("SdyService.%s rpc_env=%s tenant_id=%d date=%s: %w", method, environment, request.TenantID, request.Date, cause)
}

func recoverRPC(method string, request digitaltwin.Request, target *error) {
	if recovered := recover(); recovered != nil {
		*target = rpcError(method, request, fmt.Errorf("client panic: %v", recovered))
	}
}

var _ digitaltwin.Provider = (*Provider)(nil)

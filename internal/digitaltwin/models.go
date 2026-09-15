package digitaltwin

import (
	"context"
	"time"
)

var ErrInvalidTenant = errorString("digital twin tenant id must be positive")

type errorString string

func (e errorString) Error() string { return string(e) }

type Request struct {
	TenantID int64
	Date     string
}

type Overview struct {
	ExpectedArrival int64 `json:"expected_arrival"`
	Arrived         int64 `json:"arrived"`
	NoConsult       int64 `json:"no_consult"`
	NeedConsult     int64 `json:"need_consult"`
	NonQuick        int64 `json:"non_quick"`
}

type DutyStaff struct {
	Consultants int64 `json:"consultants"`
	Nurses      int64 `json:"nurses"`
	Doctors     int64 `json:"doctors"`
}

type TrafficFlow struct {
	ReceptionCurrent           int64 `json:"reception_current"`
	ConsultationCurrent        int64 `json:"consultation_current"`
	ConsultationServed         int64 `json:"consultation_served"`
	Waiting                    int64 `json:"waiting"`
	WaitingNoConsult           int64 `json:"waiting_no_consult"`
	WaitingNeedConsult         int64 `json:"waiting_need_consult"`
	WaitingNonQuick            int64 `json:"waiting_non_quick"`
	TreatmentCurrent           int64 `json:"treatment_current"`
	TreatmentServed            int64 `json:"treatment_served"`
	TreatmentServedNoConsult   int64 `json:"treatment_served_no_consult"`
	TreatmentServedNeedConsult int64 `json:"treatment_served_need_consult"`
	TreatmentServedNonQuick    int64 `json:"treatment_served_non_quick"`
	PostoperativeCare          int64 `json:"postoperative_care"`
}

type Dashboard struct {
	TenantID  int64       `json:"tenant_id"`
	Date      string      `json:"date"`
	FetchedAt time.Time   `json:"fetched_at"`
	Overview  Overview    `json:"overview"`
	DutyStaff DutyStaff   `json:"duty_staff"`
	Traffic   TrafficFlow `json:"traffic_flow"`
}

// Provider is the only boundary that knows how to call SdyService. The HTTP
// layer and the dashboard mapping intentionally depend on this small contract
// so generated Triple code can be replaced without changing product logic.
type Provider interface {
	GetOverview(ctx context.Context, request Request) (Overview, error)
	GetDutyStaff(ctx context.Context, request Request) (DutyStaff, error)
	GetTrafficFlow(ctx context.Context, request Request) (TrafficFlow, error)
}

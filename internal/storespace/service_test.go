package storespace

import (
	"context"
	"errors"
	"testing"
)

func TestCreateStoreRequiresDesignPlanOrRecorder(t *testing.T) {
	service := NewService(NewMemoryStore())

	_, err := service.CreateStore(context.Background(), CreateStoreInput{
		Name: "深圳壹方城",
	})

	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["resources"] != "请至少上传设计图或填写一个录像机设备编码" {
		t.Fatalf("unexpected resources error: %#v", validationError.Fields)
	}
}

func TestCreateStoreAcceptsDesignPlanOnly(t *testing.T) {
	service := NewService(NewMemoryStore())

	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		Name:               "深圳壹方城",
		DesignPlanUploadID: "upload_123",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if store.ID == 0 {
		t.Fatal("expected store id")
	}
	if store.DesignPlanStatus != DesignPlanStatusPendingRecognition {
		t.Fatalf("expected pending recognition design plan status, got %q", store.DesignPlanStatus)
	}
	if len(store.DesignPlans) != 1 {
		t.Fatalf("expected one design plan, got %d", len(store.DesignPlans))
	}
}

func TestCreateStoreAcceptsRecordersOnlyAndRejectsDuplicateCodes(t *testing.T) {
	service := NewService(NewMemoryStore())

	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
			{DeviceCode: "D87654321"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if len(store.Recorders) != 2 {
		t.Fatalf("expected two recorders, got %d", len(store.Recorders))
	}

	_, err = service.CreateStore(context.Background(), CreateStoreInput{
		Name: "广州天河",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
		},
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["recorders[0].device_code"] != "录像机设备编码已存在" {
		t.Fatalf("unexpected recorder error: %#v", validationError.Fields)
	}
}

func TestCreateStoreRejectsDuplicateRecorderCodesInRequest(t *testing.T) {
	service := NewService(NewMemoryStore())

	_, err := service.CreateStore(context.Background(), CreateStoreInput{
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
			{DeviceCode: " D12345678 "},
		},
	})

	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["recorders[1].device_code"] != "同一门店内录像机设备编码不能重复" {
		t.Fatalf("unexpected recorder error: %#v", validationError.Fields)
	}
}

func TestFindOrCreateAreaRequiresNumberAndEnforcesUniqueness(t *testing.T) {
	service := NewService(NewMemoryStore())
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	first, err := service.FindOrCreateArea(context.Background(), AreaLookup{
		StoreID:    store.ID,
		Type:       AreaTypeConsultation,
		NumberText: "2",
		Source:     AreaSourceVideoChannel,
	})
	if err != nil {
		t.Fatalf("find or create area: %v", err)
	}
	second, err := service.FindOrCreateArea(context.Background(), AreaLookup{
		StoreID:    store.ID,
		Type:       AreaTypeConsultation,
		NumberText: "2",
		Source:     AreaSourceDesignPlan,
	})
	if err != nil {
		t.Fatalf("find existing area: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected existing area id %d, got %d", first.ID, second.ID)
	}

	_, err = service.FindOrCreateArea(context.Background(), AreaLookup{
		StoreID:    store.ID,
		Type:       AreaTypeBeauty,
		NumberText: "",
		Source:     AreaSourceManual,
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["area_number"] != "区域编号必填" {
		t.Fatalf("unexpected area number error: %#v", validationError.Fields)
	}
}

package storespace

import (
	"context"
	"database/sql"
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
	if validationError.Fields["city"] != "城市必填" {
		t.Fatalf("unexpected city error: %#v", validationError.Fields)
	}
}

func TestCreateStoreAcceptsDesignPlanOnly(t *testing.T) {
	service := NewService(NewMemoryStore())

	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City:               "深圳",
		Name:               "深圳壹方城",
		DesignPlanUploadID: "upload_123",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if store.ID == 0 {
		t.Fatal("expected store id")
	}
	if store.City != "深圳" {
		t.Fatalf("expected city to be saved, got %q", store.City)
	}
	if store.DesignPlanStatus != DesignPlanStatusPendingRecognition {
		t.Fatalf("expected pending recognition design plan status, got %q", store.DesignPlanStatus)
	}
	if len(store.DesignPlans) != 1 {
		t.Fatalf("expected one design plan, got %d", len(store.DesignPlans))
	}
}

func TestListStoresIncludesCity(t *testing.T) {
	service := NewService(NewMemoryStore())

	_, err := service.CreateStore(context.Background(), CreateStoreInput{
		City:               "深圳",
		Name:               "深圳壹方城",
		DesignPlanUploadID: "upload_123",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	result, err := service.ListStores(context.Background(), StoreFilters{})
	if err != nil {
		t.Fatalf("list stores: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 store, got %d", len(result.Items))
	}
	if result.Items[0].City != "深圳" {
		t.Fatalf("expected city in list item, got %q", result.Items[0].City)
	}
}

func TestCreateStoreAcceptsRecordersOnlyAndRejectsDuplicateCodes(t *testing.T) {
	service := NewService(NewMemoryStore())

	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
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
		City: "广州",
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

func TestDeleteRecorderReleasesDeviceCode(t *testing.T) {
	service := NewService(NewMemoryStore())
	ctx := context.Background()

	store, err := service.CreateStore(ctx, CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if err := service.DeleteRecorder(ctx, store.Recorders[0].ID); err != nil {
		t.Fatalf("delete recorder: %v", err)
	}

	nextStore, err := service.CreateStore(ctx, CreateStoreInput{
		City: "广州",
		Name: "广州天河",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
		},
	})
	if err != nil {
		t.Fatalf("expected device code to be reusable after delete, got %v", err)
	}
	if len(nextStore.Recorders) != 1 || nextStore.Recorders[0].DeviceCode != "D12345678" {
		t.Fatalf("unexpected recorders: %#v", nextStore.Recorders)
	}
}

func TestAddRecorderAfterDelete(t *testing.T) {
	service := NewService(NewMemoryStore())
	ctx := context.Background()

	store, err := service.CreateStore(ctx, CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := service.DeleteRecorder(ctx, store.Recorders[0].ID); err != nil {
		t.Fatalf("delete recorder: %v", err)
	}

	updated, err := service.AddRecorder(ctx, store.ID, AddRecorderInput{DeviceCode: "D87654321"})
	if err != nil {
		t.Fatalf("add recorder: %v", err)
	}
	if len(updated.Recorders) != 1 || updated.Recorders[0].DeviceCode != "D87654321" {
		t.Fatalf("unexpected recorders after add: %#v", updated.Recorders)
	}
}

func TestAddRecorderRejectsLimitAndDuplicateCode(t *testing.T) {
	service := NewService(NewMemoryStore())
	ctx := context.Background()

	store, err := service.CreateStore(ctx, CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
			{DeviceCode: "D87654321"},
			{DeviceCode: "D99999999"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	_, err = service.AddRecorder(ctx, store.ID, AddRecorderInput{DeviceCode: "D00000000"})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["recorders"] != "单门店最多 3 台录像机" {
		t.Fatalf("unexpected limit error: %#v", validationError.Fields)
	}

	otherStore, err := service.CreateStore(ctx, CreateStoreInput{
		City: "广州",
		Name: "广州天河",
		Recorders: []RecorderInput{
			{DeviceCode: "D55555555"},
		},
	})
	if err != nil {
		t.Fatalf("create other store: %v", err)
	}
	_, err = service.AddRecorder(ctx, otherStore.ID, AddRecorderInput{DeviceCode: "D12345678"})
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["device_code"] != "录像机设备编码已存在" {
		t.Fatalf("unexpected duplicate error: %#v", validationError.Fields)
	}
}

func TestCreateStoreRejectsDuplicateRecorderCodesInRequest(t *testing.T) {
	service := NewService(NewMemoryStore())

	_, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
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

func TestScanRecorderChannelsRequiresEzvizAccount(t *testing.T) {
	service := NewServiceWithScanner(NewMemoryStore(), fakeChannelScanner{})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "GN0941203"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	_, err = service.ScanRecorderChannels(context.Background(), store.Recorders[0].ID)

	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["ezviz_account_id"] != "缺少萤石云账号" {
		t.Fatalf("unexpected validation fields: %#v", validationError.Fields)
	}
}

func TestScanRecorderChannelsStoresActiveChannelsOnly(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{
			{ChannelNo: 1, ChannelName: "通道1", Active: true},
			{ChannelNo: 2, ChannelName: "通道2", Active: true},
			{ChannelNo: 10, ChannelName: "通道10", Active: false},
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "GN0941203"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	recorder, err := service.ScanRecorderChannels(context.Background(), store.Recorders[0].ID)
	if err != nil {
		t.Fatalf("scan channels: %v", err)
	}

	if recorder.Status != RecorderStatusOnline {
		t.Fatalf("expected recorder online, got %q", recorder.Status)
	}
	if recorder.EffectiveChannelCount != 2 {
		t.Fatalf("expected 2 effective channels, got %d", recorder.EffectiveChannelCount)
	}
	if recorder.LastScannedAt == nil {
		t.Fatal("expected last scanned time")
	}
	if len(recorder.Channels) != 2 {
		t.Fatalf("expected only active channels, got %#v", recorder.Channels)
	}
	if recorder.Channels[0].ChannelNo != 1 || recorder.Channels[1].ChannelNo != 2 {
		t.Fatalf("expected exact channel numbers 1 and 2, got %#v", recorder.Channels)
	}

	updatedStore, err := service.GetStore(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get store after scan: %v", err)
	}
	if len(updatedStore.Recorders) != 1 {
		t.Fatalf("expected one recorder, got %#v", updatedStore.Recorders)
	}
	if len(updatedStore.Recorders[0].Channels) != 2 {
		t.Fatalf("expected scanned channels in store detail, got %#v", updatedStore.Recorders[0].Channels)
	}
}

func TestConfirmChannelCreatesBusinessArea(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华南"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "GQ2603603"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	recorder, err := service.ScanRecorderChannels(context.Background(), store.Recorders[0].ID)
	if err != nil {
		t.Fatalf("scan channels: %v", err)
	}

	updated, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
		SceneType:   SceneTypeTreatment,
	})
	if err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	if len(updated.Areas) != 1 {
		t.Fatalf("expected one business area, got %#v", updated.Areas)
	}
	if updated.Areas[0].Type != AreaTypeTreatment || updated.Areas[0].Number != 1 || updated.Areas[0].Source != AreaSourceVideoChannel {
		t.Fatalf("unexpected area after confirm: %#v", updated.Areas[0])
	}
	if len(updated.Recorders) != 1 || len(updated.Recorders[0].Channels) != 1 {
		t.Fatalf("expected channel in updated store: %#v", updated.Recorders)
	}
	channel := updated.Recorders[0].Channels[0]
	if channel.Status != ChannelStatusConfirmedBusiness {
		t.Fatalf("expected confirmed business channel, got %q", channel.Status)
	}
	if channel.AreaID != updated.Areas[0].ID || channel.AreaType != AreaTypeTreatment || channel.AreaNumber != 1 {
		t.Fatalf("expected channel linked to area, got %#v", channel)
	}
	if channel.ConfirmedAt == nil {
		t.Fatal("expected confirmation timestamp")
	}
}

type fakeChannelScanner struct {
	channels []ScannedChannel
}

func (f fakeChannelScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	return f.channels, nil
}

func TestFindOrCreateAreaRequiresNumberAndEnforcesUniqueness(t *testing.T) {
	service := NewService(NewMemoryStore())
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
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

func TestParseAreaBoxReturnsAnnotationCoordinates(t *testing.T) {
	box, ok := parseAreaBox(
		sql.NullString{String: "0.12", Valid: true},
		sql.NullString{String: "0.23", Valid: true},
		sql.NullString{String: "0.34", Valid: true},
		sql.NullString{String: "0.45", Valid: true},
	)
	if !ok {
		t.Fatal("expected box to parse")
	}
	if box.X != 0.12 || box.Y != 0.23 || box.Width != 0.34 || box.Height != 0.45 {
		t.Fatalf("unexpected box: %#v", box)
	}

	if _, ok := parseAreaBox(sql.NullString{}, sql.NullString{String: "0.23", Valid: true}, sql.NullString{String: "0.34", Valid: true}, sql.NullString{String: "0.45", Valid: true}); ok {
		t.Fatal("expected missing coordinate to skip box")
	}
}

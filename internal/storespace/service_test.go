package storespace

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
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

func TestUpdateStoreBasicInfoUpdatesEditableFields(t *testing.T) {
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

	updated, err := service.UpdateStoreBasicInfo(context.Background(), store.ID, UpdateStoreBasicInfoInput{
		City:          "广州",
		Name:          "广州天河店",
		ExternalOrgID: "888001",
	})
	if err != nil {
		t.Fatalf("update store basic info: %v", err)
	}

	if updated.City != "广州" || updated.Name != "广州天河店" || updated.ExternalOrgID != "888001" {
		t.Fatalf("unexpected updated store: %#v", updated)
	}
	if len(updated.Recorders) != 1 || updated.Recorders[0].DeviceCode != "D12345678" {
		t.Fatalf("expected existing recorders to remain unchanged, got %#v", updated.Recorders)
	}
}

func TestUpdateStoreBasicInfoRejectsDuplicateName(t *testing.T) {
	service := NewService(NewMemoryStore())

	first, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{DeviceCode: "D12345678"},
		},
	})
	if err != nil {
		t.Fatalf("create first store: %v", err)
	}
	_, err = service.CreateStore(context.Background(), CreateStoreInput{
		City: "广州",
		Name: "广州天河店",
		Recorders: []RecorderInput{
			{DeviceCode: "D87654321"},
		},
	})
	if err != nil {
		t.Fatalf("create second store: %v", err)
	}

	_, err = service.UpdateStoreBasicInfo(context.Background(), first.ID, UpdateStoreBasicInfoInput{
		City: "深圳",
		Name: "广州天河店",
	})

	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["name"] != "已存在同名门店" {
		t.Fatalf("unexpected fields: %#v", validationError.Fields)
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

func TestSyncEzvizAccountNamesCreatesPublicRegionAccounts(t *testing.T) {
	repo := NewMemoryStore()
	service := NewService(repo)

	if err := service.SyncEzvizAccountNames(context.Background(), []string{"华北", "华东", "华北", " "}); err != nil {
		t.Fatalf("sync account names: %v", err)
	}

	accounts, err := service.ListEzvizAccounts(context.Background())
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %#v", accounts)
	}
	if accounts[0].AccountName != "华东" || accounts[1].AccountName != "华北" {
		t.Fatalf("unexpected accounts: %#v", accounts)
	}
	for _, account := range accounts {
		if account.Status != "available" {
			t.Fatalf("expected synced account to be available, got %#v", account)
		}
	}

	if err := service.SyncEzvizAccountNames(context.Background(), []string{"华南"}); err != nil {
		t.Fatalf("sync second account names: %v", err)
	}
	accounts, err = service.ListEzvizAccounts(context.Background())
	if err != nil {
		t.Fatalf("list accounts after second sync: %v", err)
	}
	if len(accounts) != 3 {
		t.Fatalf("expected sync to append missing account only, got %#v", accounts)
	}
}

func TestEzvizAccountsFromEnvExtractsAccountNames(t *testing.T) {
	t.Setenv("EZVIZ_ACCOUNTS_JSON", `[
		{"name":"华北","app_key":"north-key","app_secret":"north-secret","access_token":"north-token"},
		{"account_name":"华南","app_key":"south-key","app_secret":"south-secret"}
	]`)

	accounts, enabled, err := EzvizAccountsFromEnv()
	if err != nil {
		t.Fatalf("parse env accounts: %v", err)
	}
	if !enabled {
		t.Fatal("expected ezviz accounts env to be enabled")
	}
	names := EzvizAccountNames(accounts)
	if len(names) != 2 || names[0] != "华北" || names[1] != "华南" {
		t.Fatalf("unexpected names: %#v", names)
	}
}

func TestScanRecorderChannelsPreservesConfirmedChannelMappings(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &mutableFakeChannelScanner{
		channels: []ScannedChannel{
			{ChannelNo: 1, ChannelName: "通道1", Active: true},
			{ChannelNo: 2, ChannelName: "通道2", Active: true},
		},
	}
	service := NewServiceWithScanner(repo, scanner)
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	}); err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	scanner.channels = []ScannedChannel{
		{ChannelNo: 1, ChannelName: "通道1-新名称", Active: true},
		{ChannelNo: 2, ChannelName: "通道2", Active: false},
		{ChannelNo: 3, ChannelName: "通道3", Active: true},
	}
	rescanned, err := service.ScanRecorderChannels(context.Background(), recorder.ID)
	if err != nil {
		t.Fatalf("rescan channels: %v", err)
	}

	if len(rescanned.Channels) != 3 {
		t.Fatalf("expected three tracked channels after rescan, got %#v", rescanned.Channels)
	}
	byNo := channelsByNo(rescanned.Channels)
	if byNo[1].Status != ChannelStatusConfirmedBusiness || byNo[1].AreaType != AreaTypeTreatment || byNo[1].AreaNumber != 1 {
		t.Fatalf("expected confirmed channel 1 mapping to be preserved, got %#v", byNo[1])
	}
	if byNo[2].Status != ChannelStatusInactive || byNo[2].IsActive {
		t.Fatalf("expected channel 2 to become inactive, got %#v", byNo[2])
	}
	if byNo[3].Status != ChannelStatusPendingRecognition || !byNo[3].IsActive {
		t.Fatalf("expected new active channel 3 to be pending recognition, got %#v", byNo[3])
	}
	if rescanned.EffectiveChannelCount != 2 {
		t.Fatalf("expected two effective channels, got %d", rescanned.EffectiveChannelCount)
	}
}

func TestScanRecorderChannelsPreservesConfirmedMappingAfterChannelRecovers(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &mutableFakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	}
	service := NewServiceWithScanner(repo, scanner)
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	}); err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	scanner.channels = []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: false}}
	if _, err := service.ScanRecorderChannels(context.Background(), recorder.ID); err != nil {
		t.Fatalf("scan inactive channel: %v", err)
	}
	scanner.channels = []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1恢复", Active: true}}
	recovered, err := service.ScanRecorderChannels(context.Background(), recorder.ID)
	if err != nil {
		t.Fatalf("scan recovered channel: %v", err)
	}

	channel := recovered.Channels[0]
	if channel.Status != ChannelStatusConfirmedBusiness || channel.AreaType != AreaTypeTreatment || channel.AreaNumber != 1 {
		t.Fatalf("expected recovered channel to keep confirmed mapping, got %#v", channel)
	}
}

func TestDeleteChannelAllowsRescanToRecreateAsPending(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &mutableFakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	}
	service := NewServiceWithScanner(repo, scanner)
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	}); err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	afterDelete, err := service.DeleteChannel(context.Background(), recorder.Channels[0].ID)
	if err != nil {
		t.Fatalf("delete channel: %v", err)
	}
	if len(afterDelete.Recorders[0].Channels) != 0 {
		t.Fatalf("expected channel deleted, got %#v", afterDelete.Recorders[0].Channels)
	}

	rescanned, err := service.ScanRecorderChannels(context.Background(), recorder.ID)
	if err != nil {
		t.Fatalf("rescan channels: %v", err)
	}
	if len(rescanned.Channels) != 1 {
		t.Fatalf("expected channel to be recreated, got %#v", rescanned.Channels)
	}
	channel := rescanned.Channels[0]
	if channel.ChannelNo != 1 || channel.Status != ChannelStatusPendingRecognition || channel.AreaType != "" || channel.AreaNumber != 0 {
		t.Fatalf("expected recreated channel to be pending and unconfirmed, got %#v", channel)
	}
}

func TestRecognizeRecorderChannelsCapturesSnapshots(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
		snapshots: map[int]string{
			1: "https://example.test/channel-1.jpg",
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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

	updated, err := service.RecognizeRecorderChannels(context.Background(), recorder.ID)
	if err != nil {
		t.Fatalf("recognize recorder channels: %v", err)
	}

	if len(updated.Channels) != 1 {
		t.Fatalf("expected one channel, got %#v", updated.Channels)
	}
	if updated.Channels[0].ThumbnailURL != "https://example.test/channel-1.jpg" {
		t.Fatalf("expected snapshot URL on channel, got %#v", updated.Channels[0])
	}
	if updated.Channels[0].RecognitionAttempts != 1 {
		t.Fatalf("expected one recognition attempt, got %#v", updated.Channels[0])
	}
}

func TestRecognizeRecorderChannelsStoresRemoteSnapshotsLocally(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
		snapshots: map[int]string{
			1: "https://opencapture.ys7.com/snapshot.jpg?Expires=1",
		},
	})
	snapshotStore := NewLocalSnapshotStore(t.TempDir())
	snapshotStore.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(strings.NewReader("fake-jpeg-data")),
			Request:    request,
		}, nil
	})}
	service.snapshotStore = snapshotStore
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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

	updated, err := service.RecognizeRecorderChannels(context.Background(), recorder.ID)
	if err != nil {
		t.Fatalf("recognize recorder channels: %v", err)
	}

	if len(updated.Channels) != 1 {
		t.Fatalf("expected one channel, got %#v", updated.Channels)
	}
	if !strings.HasPrefix(updated.Channels[0].ThumbnailURL, "/api/store-space/channel-snapshots/") {
		t.Fatalf("expected locally served thumbnail URL, got %#v", updated.Channels[0])
	}
	if strings.Contains(updated.Channels[0].ThumbnailURL, "Expires=") {
		t.Fatalf("expected thumbnail URL not to expose expiring remote query, got %q", updated.Channels[0].ThumbnailURL)
	}
}

func TestRefreshChannelSnapshotKeepsConfirmedMapping(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
		snapshots: map[int]string{
			1: "https://opencapture.ys7.com/snapshot.jpg?Expires=1",
		},
	})
	snapshotStore := NewLocalSnapshotStore(t.TempDir())
	snapshotStore.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(strings.NewReader("fake-jpeg-data")),
			Request:    request,
		}, nil
	})}
	service.snapshotStore = snapshotStore
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	confirmed, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "2",
	})
	if err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	updated, err := service.RefreshChannelSnapshot(context.Background(), confirmed.Recorders[0].Channels[0].ID)
	if err != nil {
		t.Fatalf("refresh snapshot: %v", err)
	}

	if updated.Status != ChannelStatusConfirmedBusiness || updated.AreaType != AreaTypeTreatment || updated.AreaNumber != 2 {
		t.Fatalf("expected confirmed mapping to stay unchanged, got %#v", updated)
	}
	if updated.RecognitionAttempts != 0 {
		t.Fatalf("expected refresh not to count as recognition attempt, got %#v", updated)
	}
	if !strings.HasPrefix(updated.ThumbnailURL, "/api/store-space/channel-snapshots/") {
		t.Fatalf("expected local snapshot URL, got %#v", updated)
	}
}

func TestExpiredRemoteChannelSnapshotIsNotExposed(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	expiredAt := time.Now().Add(-time.Hour)
	_, err = repo.SaveChannelSnapshot(context.Background(), recorder.Channels[0].ID, ChannelSnapshotInput{
		ThumbnailPath:       "https://opencapture.ys7.com/snapshot.jpg?Expires=1",
		FullImagePath:       "https://opencapture.ys7.com/snapshot.jpg?Expires=1",
		FullImageExpiresAt:  &expiredAt,
		RecognitionResult:   channelRecognitionStatusJSON("captured", "", 10, 0, 10),
		AreaNumberText:      "1",
		CountAttempt:        true,
	})
	if err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	loaded, err := service.GetStore(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	channel := loaded.Recorders[0].Channels[0]
	if channel.ThumbnailURL != "" || channel.FullImageURL != "" {
		t.Fatalf("expected expired remote snapshot URLs to be hidden, got thumbnail=%q full=%q", channel.ThumbnailURL, channel.FullImageURL)
	}
	if channel.FullImageExpiresAt == nil {
		t.Fatal("expected expiration metadata to remain available")
	}
}

func TestProbeRecognizeChannelCreatesChannelAndStoresRecognition(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华东"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScannerAndRecognizer(repo, fakeChannelScanner{
		snapshots: map[int]string{
			1: "https://opencapture.ys7.com/snapshot.jpg?Expires=1",
		},
	}, fakeChannelRecognizer{
		result: ChannelRecognitionResult{
			SceneType:  string(SceneTypeTreatment),
			AreaType:   string(AreaTypeTreatment),
			AreaNumber: "3",
			Confidence: "high",
		},
	})
	snapshotStore := NewLocalSnapshotStore(t.TempDir())
	snapshotStore.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(strings.NewReader("fake-jpeg-data")),
			Request:    request,
		}, nil
	})}
	service.UseSnapshotStore(snapshotStore)
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "上海",
		Name: "上海测试店",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "K92940413"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	result, err := service.ProbeRecognizeChannel(context.Background(), store.Recorders[0].ID, ProbeRecognizeChannelInput{ChannelNo: 1})
	if err != nil {
		t.Fatalf("probe recognize channel: %v", err)
	}

	if !result.Active || result.Channel == nil {
		t.Fatalf("expected active channel, got %#v", result)
	}
	if result.Channel.ChannelNo != 1 {
		t.Fatalf("expected channel 1, got %#v", result.Channel)
	}
	if result.Channel.Status != ChannelStatusPendingConfirmation {
		t.Fatalf("expected pending confirmation, got %#v", result.Channel)
	}
	if result.Channel.AreaType != AreaTypeTreatment || result.Channel.AreaNumber != 3 {
		t.Fatalf("expected treatment 3, got %#v", result.Channel)
	}
	if !strings.HasPrefix(result.Channel.ThumbnailURL, "/api/store-space/channel-snapshots/") {
		t.Fatalf("expected stable snapshot URL, got %#v", result.Channel)
	}
	if result.Channel.RecognitionAttempts != 1 {
		t.Fatalf("expected one recognition attempt, got %#v", result.Channel)
	}
	updated, err := service.GetStore(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	if len(updated.Recorders[0].Channels) != 1 || updated.Recorders[0].EffectiveChannelCount != 1 {
		t.Fatalf("expected recorder metrics updated, got %#v", updated.Recorders[0])
	}
}

func TestProbeRecognizeChannelReturnsInactiveWhenCaptureFails(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华东"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, failingCaptureScanner{})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "上海",
		Name: "上海测试店",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "K92940413"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	result, err := service.ProbeRecognizeChannel(context.Background(), store.Recorders[0].ID, ProbeRecognizeChannelInput{ChannelNo: 11})
	if err != nil {
		t.Fatalf("probe recognize channel: %v", err)
	}
	if result.Active || result.Channel != nil {
		t.Fatalf("expected inactive result, got %#v", result)
	}
	updated, err := service.GetStore(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	if len(updated.Recorders[0].Channels) != 0 {
		t.Fatalf("expected no channel created after capture failure, got %#v", updated.Recorders[0].Channels)
	}
}

func TestRecognizeRecorderChannelsPrefillsAIResultAndKeepsPendingConfirmation(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScannerAndRecognizer(repo, fakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
		snapshots: map[int]string{
			1: "https://example.test/channel-1.jpg",
		},
	}, fakeChannelRecognizer{
		result: ChannelRecognitionResult{
			SceneType:      string(SceneTypeTreatment),
			AreaType:       string(AreaTypeTreatment),
			AreaNumber:     "2",
			CardText:       "治疗室 2",
			DecisionSource: "number_card",
			Confidence:     "high",
			RawNotes:       "编号卡片显示治疗室 2",
			Provider:       "test-provider",
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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

	updated, err := service.RecognizeRecorderChannels(context.Background(), recorder.ID)
	if err != nil {
		t.Fatalf("recognize recorder channels: %v", err)
	}

	channel := updated.Channels[0]
	if channel.Status != ChannelStatusPendingConfirmation {
		t.Fatalf("expected pending confirmation, got %#v", channel)
	}
	if channel.AreaType != AreaTypeTreatment || channel.AreaNumber != 2 || channel.SceneType != SceneTypeTreatment {
		t.Fatalf("expected AI prefill from number card, got %#v", channel)
	}
	if !strings.Contains(channel.RecognitionResult, `"decision_source":"number_card"`) {
		t.Fatalf("expected recognition result to include decision source, got %s", channel.RecognitionResult)
	}
	if !strings.Contains(channel.RecognitionResult, `"recognition_ms"`) {
		t.Fatalf("expected timing metrics in recognition result, got %s", channel.RecognitionResult)
	}
	if !strings.Contains(channel.RecognitionResult, `"provider":"test-provider"`) {
		t.Fatalf("expected recognition provider in recognition result, got %s", channel.RecognitionResult)
	}
}

func TestRecognizeRecorderChannelsSkipsConfirmedChannel(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &countingFakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	}
	service := NewServiceWithScannerAndRecognizer(repo, scanner, fakeChannelRecognizer{
		result: ChannelRecognitionResult{
			SceneType:      string(SceneTypeConsultation),
			AreaType:       string(AreaTypeConsultation),
			AreaNumber:     "9",
			CardText:       "面诊室 9",
			DecisionSource: "number_card",
			Confidence:     "high",
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	}); err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	updated, err := service.RecognizeRecorderChannels(context.Background(), recorder.ID)
	if err != nil {
		t.Fatalf("recognize recorder channels: %v", err)
	}

	channel := updated.Channels[0]
	if channel.Status != ChannelStatusConfirmedBusiness {
		t.Fatalf("expected confirmed channel to stay locked, got %#v", channel)
	}
	if channel.AreaType != AreaTypeTreatment || channel.AreaNumber != 1 {
		t.Fatalf("expected confirmed business mapping to stay unchanged, got %#v", channel)
	}
	if scanner.captureCount != 0 {
		t.Fatalf("expected confirmed channel to be skipped, got capture count %d", scanner.captureCount)
	}
}

func TestRecognizeChannelRejectsConfirmedChannel(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &countingFakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	}
	service := NewServiceWithScannerAndRecognizer(repo, scanner, fakeChannelRecognizer{
		result: ChannelRecognitionResult{
			SceneType:      string(SceneTypeConsultation),
			AreaType:       string(AreaTypeConsultation),
			AreaNumber:     "9",
			DecisionSource: "number_card",
			Confidence:     "high",
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	}); err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	_, err = service.RecognizeChannel(context.Background(), recorder.Channels[0].ID)
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["channel"] == "" {
		t.Fatalf("expected confirmed channel validation error, got %v", err)
	}
	if scanner.captureCount != 0 {
		t.Fatalf("expected confirmed channel not to be captured, got count %d", scanner.captureCount)
	}
}

func TestUnlockChannelForEditAllowsRecognitionAgain(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &countingFakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	}
	service := NewServiceWithScannerAndRecognizer(repo, scanner, fakeChannelRecognizer{
		result: ChannelRecognitionResult{
			SceneType:      string(SceneTypeConsultation),
			AreaType:       string(AreaTypeConsultation),
			AreaNumber:     "2",
			DecisionSource: "number_card",
			Confidence:     "high",
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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
	confirmed, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	})
	if err != nil {
		t.Fatalf("confirm channel: %v", err)
	}
	if confirmed.Recorders[0].Channels[0].Status != ChannelStatusConfirmedBusiness {
		t.Fatalf("expected confirmed channel, got %#v", confirmed.Recorders[0].Channels[0])
	}

	unlocked, err := service.UnlockChannelForEdit(context.Background(), recorder.Channels[0].ID)
	if err != nil {
		t.Fatalf("unlock channel: %v", err)
	}
	if unlocked.Status != ChannelStatusPendingConfirmation {
		t.Fatalf("expected pending confirmation after unlock, got %#v", unlocked)
	}
	if unlocked.ConfirmedAt != nil {
		t.Fatalf("expected confirmed timestamp cleared, got %#v", unlocked.ConfirmedAt)
	}
	if unlocked.AreaType != AreaTypeTreatment || unlocked.AreaNumber != 1 {
		t.Fatalf("expected mapping draft to be preserved, got %#v", unlocked)
	}

	recognized, err := service.RecognizeChannel(context.Background(), recorder.Channels[0].ID)
	if err != nil {
		t.Fatalf("recognize unlocked channel: %v", err)
	}
	if recognized.AreaType != AreaTypeConsultation || recognized.AreaNumber != 2 {
		t.Fatalf("expected unlocked channel to be recognized again, got %#v", recognized)
	}
}

func TestRecognizeChannelOnlyUpdatesOneChannel(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &countingFakeChannelScanner{
		channels: []ScannedChannel{
			{ChannelNo: 1, ChannelName: "通道1", Active: true},
			{ChannelNo: 2, ChannelName: "通道2", Active: true},
		},
	}
	service := NewServiceWithScannerAndRecognizer(repo, scanner, fakeChannelRecognizer{
		result: ChannelRecognitionResult{
			SceneType:      string(SceneTypeBeauty),
			AreaType:       string(AreaTypeBeauty),
			AreaNumber:     "3",
			DecisionSource: "number_card",
			Confidence:     "high",
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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

	updatedChannel, err := service.RecognizeChannel(context.Background(), recorder.Channels[1].ID)
	if err != nil {
		t.Fatalf("recognize channel: %v", err)
	}

	if scanner.captureCount != 1 || scanner.capturedChannelNos[0] != 2 {
		t.Fatalf("expected only channel 2 captured, got count=%d channels=%#v", scanner.captureCount, scanner.capturedChannelNos)
	}
	if updatedChannel.ChannelNo != 2 || updatedChannel.AreaType != AreaTypeBeauty || updatedChannel.AreaNumber != 3 {
		t.Fatalf("unexpected recognized channel: %#v", updatedChannel)
	}
}

func TestRecognizeChannelStoresNonBusinessSceneAsNote(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	scanner := &countingFakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}},
	}
	service := NewServiceWithScannerAndRecognizer(repo, scanner, fakeChannelRecognizer{
		result: ChannelRecognitionResult{
			SceneType:      string(SceneTypeMachineRoom),
			AreaType:       "",
			AreaNumber:     "机房",
			DecisionSource: "scene",
			Confidence:     "high",
		},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "北京",
		Name: "北京测试店",
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

	updatedChannel, err := service.RecognizeChannel(context.Background(), recorder.Channels[0].ID)
	if err != nil {
		t.Fatalf("recognize channel: %v", err)
	}

	if updatedChannel.AreaType != "" || updatedChannel.AreaNumber != 0 {
		t.Fatalf("expected non-business channel without business area number, got %#v", updatedChannel)
	}
	if updatedChannel.SceneType != SceneTypeMachineRoom || updatedChannel.AreaNote != "机房" || updatedChannel.Status != ChannelStatusPendingConfirmation {
		t.Fatalf("expected machine room note pending confirmation, got %#v", updatedChannel)
	}
}

func TestExportChannelMappingExcelExportsActiveChannelsInBusinessOrder(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{
			{ChannelNo: 1, ChannelName: "治疗", Active: true},
			{ChannelNo: 2, ChannelName: "面诊", Active: true},
			{ChannelNo: 3, ChannelName: "生美", Active: true},
			{ChannelNo: 4, ChannelName: "机房", Active: true},
			{ChannelNo: 5, ChannelName: "失效", Active: true},
		},
	})
	service.UseSnapshotStore(memorySnapshotStore{
		files: map[string][]byte{"00000000000000000000000000000001.jpg": []byte("fake-jpeg")},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City:          "深圳",
		Name:          "深圳壹方城",
		ExternalOrgID: "10001",
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
	if _, err := repo.SaveChannelSnapshot(context.Background(), recorder.Channels[0].ID, ChannelSnapshotInput{ThumbnailPath: "/api/store-space/channel-snapshots/00000000000000000000000000000001.jpg", FullImagePath: "/api/store-space/channel-snapshots/00000000000000000000000000000001.jpg"}); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{AreaType: AreaTypeTreatment, AreaNumber: "2"}); err != nil {
		t.Fatalf("confirm treatment: %v", err)
	}
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[1].ID, ChannelConfirmationInput{AreaType: AreaTypeConsultation, AreaNumber: "1"}); err != nil {
		t.Fatalf("confirm consultation: %v", err)
	}
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[2].ID, ChannelConfirmationInput{AreaType: AreaTypeBeauty, AreaNumber: "3"}); err != nil {
		t.Fatalf("confirm beauty: %v", err)
	}
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[3].ID, ChannelConfirmationInput{SceneType: SceneTypeMachineRoom, AreaNote: "机房"}); err != nil {
		t.Fatalf("confirm machine room: %v", err)
	}
	if _, err := service.DeleteChannel(context.Background(), recorder.Channels[4].ID); err != nil {
		t.Fatalf("delete channel: %v", err)
	}

	exported, err := service.ExportChannelMappingExcel(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("export excel: %v", err)
	}

	if exported.ContentType != channelMappingExcelContentType {
		t.Fatalf("unexpected content type: %s", exported.ContentType)
	}
	if !strings.HasPrefix(exported.FileName, "深圳壹方城-通道映射确认表-") || !strings.HasSuffix(exported.FileName, ".xlsx") {
		t.Fatalf("unexpected filename: %s", exported.FileName)
	}
	files := unzipExcelFiles(t, exported.Content)
	sheet := files["xl/worksheets/sheet1.xml"]
	for _, want := range []string{"面诊室", "治疗室", "生美", "其他区域", "10001", "GN0941203", "机房"} {
		if !strings.Contains(sheet, want) {
			t.Fatalf("sheet missing %q: %s", want, sheet)
		}
	}
	if strings.Contains(sheet, ">5<") {
		t.Fatalf("inactive/deleted channel should not be exported: %s", sheet)
	}
	if strings.Index(sheet, "面诊室") > strings.Index(sheet, "治疗室") {
		t.Fatalf("expected consultation before treatment: %s", sheet)
	}
	if _, ok := files["xl/media/image1.jpg"]; !ok {
		t.Fatalf("expected embedded image, files=%v", mapKeys(files))
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
		SceneType:  SceneTypeTreatment,
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

func TestConfirmChannelReplacingVideoOnlyAreaRemovesOldArea(t *testing.T) {
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

	first, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	})
	if err != nil {
		t.Fatalf("confirm first channel: %v", err)
	}
	if len(first.Areas) != 1 || first.Areas[0].Number != 1 {
		t.Fatalf("expected treatment 1 after first confirm, got %#v", first.Areas)
	}
	if _, err := service.FindOrCreateArea(context.Background(), AreaLookup{
		StoreID:    store.ID,
		Type:       AreaTypeTreatment,
		NumberText: "3",
		Source:     AreaSourceVideoChannel,
	}); err != nil {
		t.Fatalf("create stale video area: %v", err)
	}

	updated, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "2",
	})
	if err != nil {
		t.Fatalf("confirm updated channel: %v", err)
	}

	if len(updated.Areas) != 1 {
		t.Fatalf("expected stale video-only area to be removed, got %#v", updated.Areas)
	}
	if updated.Areas[0].Type != AreaTypeTreatment || updated.Areas[0].Number != 2 {
		t.Fatalf("expected only treatment 2, got %#v", updated.Areas)
	}
	channel := updated.Recorders[0].Channels[0]
	if channel.AreaID != updated.Areas[0].ID || channel.AreaNumber != 2 {
		t.Fatalf("expected channel linked to treatment 2, got channel=%#v areas=%#v", channel, updated.Areas)
	}
}

func TestConfirmChannelUpdatesExistingVideoAreaWhenNumberChanges(t *testing.T) {
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

	first, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	})
	if err != nil {
		t.Fatalf("confirm treatment 1: %v", err)
	}
	firstAreaID := first.Areas[0].ID

	updated, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "2",
	})
	if err != nil {
		t.Fatalf("change channel to treatment 2: %v", err)
	}

	if len(updated.Areas) != 1 {
		t.Fatalf("expected one updated area, got %#v", updated.Areas)
	}
	if updated.Areas[0].ID != firstAreaID {
		t.Fatalf("expected area id %d to be reused, got %#v", firstAreaID, updated.Areas[0])
	}
	if updated.Areas[0].Type != AreaTypeTreatment || updated.Areas[0].Number != 2 {
		t.Fatalf("expected existing area to become treatment 2, got %#v", updated.Areas[0])
	}
	channel := updated.Recorders[0].Channels[0]
	if channel.AreaID != firstAreaID || channel.AreaNumber != 2 {
		t.Fatalf("expected channel to stay linked to updated area, got %#v", channel)
	}
}

func TestSaveDesignPlanPreservesConfirmedChannelAreaSource(t *testing.T) {
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
	confirmed, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "1",
	})
	if err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	updated, err := service.SaveDesignPlan(context.Background(), store.ID, SaveDesignPlanInput{
		UploadID:         "upload_123",
		PDFFileName:      "shenzhen.pdf",
		PreviewImagePath: "uploads/upload_123/preview.png",
		ThumbnailPath:    "uploads/upload_123/thumbnail.png",
		PageCount:        1,
		Areas: []DesignAreaInput{
			{
				ID:          confirmed.Areas[0].ID,
				DisplayName: "治疗室 2",
				Type:        AreaTypeTreatment,
				NumberText:  "2",
				Box:         &AreaBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			},
		},
	})
	if err != nil {
		t.Fatalf("save design plan: %v", err)
	}

	if len(updated.Areas) != 1 {
		t.Fatalf("expected one area, got %#v", updated.Areas)
	}
	if updated.Areas[0].Source != AreaSourceMultiple {
		t.Fatalf("expected confirmed channel area source to stay lockable as multiple, got %#v", updated.Areas[0])
	}
	if updated.Areas[0].Type != AreaTypeTreatment || updated.Areas[0].Number != 1 {
		t.Fatalf("expected design plan save not to change confirmed channel area identity, got %#v", updated.Areas[0])
	}
	if updated.Areas[0].Box == nil {
		t.Fatalf("expected design annotation box on area, got %#v", updated.Areas[0])
	}
}

type fakeChannelScanner struct {
	channels  []ScannedChannel
	snapshots map[int]string
}

func (f fakeChannelScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	return f.channels, nil
}

func (f fakeChannelScanner) CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error) {
	url := f.snapshots[channel.ChannelNo]
	if url == "" {
		url = "https://example.test/default.jpg"
	}
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	return ChannelSnapshotInput{
		ThumbnailPath:      url,
		FullImagePath:      url,
		FullImageExpiresAt: &expiresAt,
	}, nil
}

type failingCaptureScanner struct{}

func (f failingCaptureScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	return nil, nil
}

func (f failingCaptureScanner) CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error) {
	return ChannelSnapshotInput{}, errors.New("capture failed")
}

type memorySnapshotStore struct {
	files map[string][]byte
}

func (s memorySnapshotStore) SaveRemote(ctx context.Context, imageURL string) (string, error) {
	return imageURL, nil
}

func (s memorySnapshotStore) Open(ctx context.Context, name string) (io.ReadCloser, string, error) {
	data, ok := s.files[name]
	if !ok {
		return nil, "", ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), "image/jpeg", nil
}

func unzipExcelFiles(t *testing.T, payload []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("open xlsx zip: %v", err)
	}
	files := map[string]string{}
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			t.Fatalf("open zip file %s: %v", file.Name, err)
		}
		data, err := io.ReadAll(handle)
		if closeErr := handle.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatalf("read zip file %s: %v", file.Name, err)
		}
		files[file.Name] = string(data)
	}
	return files
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type mutableFakeChannelScanner struct {
	channels []ScannedChannel
}

func (f *mutableFakeChannelScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	return f.channels, nil
}

func (f *mutableFakeChannelScanner) CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error) {
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	url := "https://example.test/channel.jpg"
	return ChannelSnapshotInput{
		ThumbnailPath:      url,
		FullImagePath:      url,
		FullImageExpiresAt: &expiresAt,
	}, nil
}

func channelsByNo(channels []Channel) map[int]Channel {
	result := map[int]Channel{}
	for _, channel := range channels {
		result[channel.ChannelNo] = channel
	}
	return result
}

type countingFakeChannelScanner struct {
	channels           []ScannedChannel
	captureCount       int
	capturedChannelNos []int
}

func (f *countingFakeChannelScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	return f.channels, nil
}

func (f *countingFakeChannelScanner) CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error) {
	f.captureCount++
	f.capturedChannelNos = append(f.capturedChannelNos, channel.ChannelNo)
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	url := fmt.Sprintf("https://example.test/channel-%d.jpg", channel.ChannelNo)
	return ChannelSnapshotInput{
		ThumbnailPath:      url,
		FullImagePath:      url,
		FullImageExpiresAt: &expiresAt,
	}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type fakeChannelRecognizer struct {
	result ChannelRecognitionResult
	err    error
}

func (f fakeChannelRecognizer) RecognizeChannel(ctx context.Context, imageURL string) (ChannelRecognitionResult, error) {
	if f.err != nil {
		return ChannelRecognitionResult{}, f.err
	}
	return f.result, nil
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

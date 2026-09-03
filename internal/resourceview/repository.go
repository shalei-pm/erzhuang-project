package resourceview

import "context"

var ErrNotFound = resourceViewError("resource view record not found")

type Repository interface {
	ListStores(ctx context.Context, filters StoreFilters) ([]StoreRecords, error)
	ListNVRMonitorStores(ctx context.Context) ([]StoreRecords, error)
	GetStoreRecords(ctx context.Context, tenantID int64) (StoreRecords, error)
	GetNVRMonitorStoreRecords(ctx context.Context, tenantID int64) (StoreRecords, error)
}

type resourceViewError string

func (e resourceViewError) Error() string { return string(e) }

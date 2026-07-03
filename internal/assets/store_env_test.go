package assets

import "testing"

func TestNewStoreFromEnvReadsOSSK8SSecretConfig(t *testing.T) {
	t.Setenv("K8S_SECRET_ASSET_STORE", "oss")
	t.Setenv("K8S_SECRET_OSS_BUCKET", "camera-assets")
	t.Setenv("K8S_SECRET_OSS_ENDPOINT", "oss-cn-beijing-internal.aliyuncs.com")
	t.Setenv("K8S_SECRET_OSS_ACCESS_KEY_ID", "access-key-id")
	t.Setenv("K8S_SECRET_OSS_ACCESS_KEY_SECRET", "access-key-secret")

	store, err := NewStoreFromEnv()
	if err != nil {
		t.Fatalf("new store from env: %v", err)
	}
	if _, ok := store.(*OSSStore); !ok {
		t.Fatalf("store = %T, want *OSSStore", store)
	}
	if mode := ModeFromEnv(); mode != "oss" {
		t.Fatalf("mode = %q, want oss", mode)
	}
}

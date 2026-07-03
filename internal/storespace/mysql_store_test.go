package storespace

import "testing"

func TestMySQLStoreSearchLikeEmptyQuery(t *testing.T) {
	for _, query := range []string{"", "   "} {
		pattern, ok := mysqlStoreSearchLike(query)
		if ok {
			t.Fatalf("mysqlStoreSearchLike(%q) ok = true, want false", query)
		}
		if pattern != "" {
			t.Fatalf("mysqlStoreSearchLike(%q) pattern = %q, want empty", query, pattern)
		}
	}
}

func TestMySQLStoreSearchLikeNormalizesQuery(t *testing.T) {
	pattern, ok := mysqlStoreSearchLike(" 北 京 10030 ")
	if !ok {
		t.Fatal("mysqlStoreSearchLike returned ok = false, want true")
	}
	if pattern != "%北京10030%" {
		t.Fatalf("mysqlStoreSearchLike pattern = %q, want %%北京10030%%", pattern)
	}
}

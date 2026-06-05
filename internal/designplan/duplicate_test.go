package designplan

import "testing"

func TestNormalizeStoreNameRemovesCommonSuffixesAndSpaces(t *testing.T) {
	got := NormalizeStoreName(" 新氧青春 杭州西湖门店 ")
	if got != "杭州西湖" {
		t.Fatalf("expected normalized name, got %q", got)
	}
}

func TestIsSimilarStoreName(t *testing.T) {
	if !IsSimilarStoreName("杭州西湖店", "新氧青春杭州西湖门店") {
		t.Fatal("expected similar names")
	}
	if IsSimilarStoreName("杭州西湖店", "上海静安店") {
		t.Fatal("expected unrelated names not to match")
	}
}

package search

import (
	"reflect"
	"testing"
)

func TestFilter(t *testing.T) {
	keys := []string{"/app/prod/DB/host", "/app/prod/db/user", "/app/staging/db/host"}

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{"no patterns", nil, keys},
		{"empty pattern", []string{""}, keys},
		{"single", []string{"staging"}, []string{"/app/staging/db/host"}},
		{"case insensitive AND", []string{"PROD", "host"}, []string{"/app/prod/DB/host"}},
		{"no match", []string{"qa"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filter(keys, tt.patterns); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestByPrefix(t *testing.T) {
	keys := []string{"/a/x", "/b/y"}
	if got := ByPrefix(keys, "/"); !reflect.DeepEqual(got, keys) {
		t.Errorf("ByPrefix(/) = %v", got)
	}
	if got := ByPrefix(keys, "/b/"); !reflect.DeepEqual(got, []string{"/b/y"}) {
		t.Errorf("ByPrefix(/b/) = %v", got)
	}
}

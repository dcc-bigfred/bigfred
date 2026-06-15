package helpers_test

import (
	"reflect"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
)

func TestMergeUserIDs(t *testing.T) {
	got := helpers.MergeUserIDs(1, 2, 2, 3, 0)
	want := []uint{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeUserIDs() = %v, want %v", got, want)
	}
}

func TestMergeUserIDsPrimaryOnly(t *testing.T) {
	got := helpers.MergeUserIDs(5)
	want := []uint{5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeUserIDs() = %v, want %v", got, want)
	}
}

func TestMergeUserIDsEmpty(t *testing.T) {
	if got := helpers.MergeUserIDs(0); got != nil {
		t.Fatalf("MergeUserIDs(0) = %v, want nil", got)
	}
}

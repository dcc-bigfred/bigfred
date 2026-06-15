package service

import "testing"

func TestUserCanDriveWithLessees(t *testing.T) {
	if !UserCanDriveWithLessees(1, 1, nil) {
		t.Fatal("owner without lease should drive")
	}
	if UserCanDriveWithLessees(1, 1, []uint{2}) {
		t.Fatal("owner with lease to other should not drive")
	}
	if !UserCanDriveWithLessees(2, 1, []uint{2}) {
		t.Fatal("lessee should drive")
	}
}

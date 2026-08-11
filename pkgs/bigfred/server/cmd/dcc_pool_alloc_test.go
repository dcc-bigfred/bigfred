package cmd

import (
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

func TestAllocateFreeDCCAddresses(t *testing.T) {
	cases := []struct {
		name     string
		count    int
		existing []domain.DCCAddressRange
		want     []PoolRange
	}{
		{
			name:  "empty pool starts at 50",
			count: 3,
			want:  []PoolRange{{From: 50, To: 52}},
		},
		{
			name:     "skips occupied addresses and merges contiguous",
			count:    4,
			existing: []domain.DCCAddressRange{{FromAddr: 51, ToAddr: 52}},
			want:     []PoolRange{{From: 50, To: 50}, {From: 53, To: 55}},
		},
		{
			name:  "splits around several occupied blocks",
			count: 5,
			existing: []domain.DCCAddressRange{
				{FromAddr: 50, ToAddr: 50},
				{FromAddr: 53, ToAddr: 53},
				{FromAddr: 56, ToAddr: 60},
			},
			want: []PoolRange{{From: 51, To: 52}, {From: 54, To: 55}, {From: 61, To: 61}},
		},
		{
			name:     "ignores addresses below the start address",
			count:    2,
			existing: []domain.DCCAddressRange{{FromAddr: 1, ToAddr: 49}},
			want:     []PoolRange{{From: 50, To: 51}},
		},
		{
			name:  "zero count allocates nothing",
			count: 0,
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := allocateFreeDCCAddresses(tc.count, tc.existing)
			if err != nil {
				t.Fatalf("allocateFreeDCCAddresses: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ranges: got %+v want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("range %d: got %+v want %+v", i, got[i], tc.want[i])
				}
			}
			var total int
			for _, r := range got {
				total += int(r.To-r.From) + 1
			}
			if total != tc.count {
				t.Fatalf("allocated %d addresses, want %d", total, tc.count)
			}
		})
	}
}

func TestAllocateFreeDCCAddressesExhausted(t *testing.T) {
	existing := []domain.DCCAddressRange{{FromAddr: 1, ToAddr: 9998}}
	if _, err := allocateFreeDCCAddresses(2, existing); !errors.Is(err, svcerrors.ErrDCCPoolExhausted) {
		t.Fatalf("expected ErrDCCPoolExhausted, got %v", err)
	}
	got, err := allocateFreeDCCAddresses(1, existing)
	if err != nil {
		t.Fatalf("allocate last free address: %v", err)
	}
	if len(got) != 1 || got[0] != (PoolRange{From: 9999, To: 9999}) {
		t.Fatalf("unexpected allocation: %+v", got)
	}
}

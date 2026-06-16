package security_test

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
)

func TestDrivePolicyCanDrive(t *testing.T) {
	t.Parallel()
	p := security.DrivePolicy{}
	vehicle := contract.AllowedVehicle{
		Addr:              31,
		ControllerUserIDs: []uint{1},
	}
	if d := p.CanDrive(1, vehicle, true); !d.Allowed {
		t.Fatalf("expected allow, got %+v", d)
	}
	if d := p.CanDrive(2, vehicle, true); d.Reason != security.ReasonNotAuthorized {
		t.Fatalf("reason = %q, want %s", d.Reason, security.ReasonNotAuthorized)
	}
	if d := p.CanDrive(1, vehicle, false); d.Reason != security.ReasonVehicleNotOnLayout {
		t.Fatalf("reason = %q, want %s", d.Reason, security.ReasonVehicleNotOnLayout)
	}
}

func TestTrainPolicyCanDriveTrain(t *testing.T) {
	t.Parallel()
	p := security.TrainPolicy{}
	train := contract.DefinedTrain{
		TrainID: 1,
		Members: []contract.DefinedTrainMember{{
			Addr: ptrUint16(31),
		}},
		ControllerUserIDs: []uint{1},
	}
	if d := p.CanDriveTrain(1, train, true); !d.Allowed {
		t.Fatalf("expected allow, got %+v", d)
	}
	if d := p.CanDriveTrain(2, train, true); d.Reason != security.ReasonNotAuthorizedToDrive {
		t.Fatalf("reason = %q", d.Reason)
	}
	if d := p.CanDriveTrain(1, train, false); d.Reason != security.ReasonTrainNotOnLayout {
		t.Fatalf("reason = %q", d.Reason)
	}
}

func ptrUint16(v uint16) *uint16 { return &v }

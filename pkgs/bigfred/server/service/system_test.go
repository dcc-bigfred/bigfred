package service

import (
	"errors"
	"testing"

	miclient "github.com/dcc-bigfred/microinit/go/client"
	"github.com/stretchr/testify/require"
)

type fakePower struct {
	info    *miclient.DaemonInfo
	infoErr error
	shutErr error
	lastMode string
}

func (f *fakePower) Info() (*miclient.DaemonInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	return f.info, nil
}

func (f *fakePower) ShutdownMode(mode string) error {
	f.lastMode = mode
	return f.shutErr
}

func TestSystemControlInfoEmptyModeIsSupervise(t *testing.T) {
	ctl := NewSystemControlWithPower(&fakePower{
		info: &miclient.DaemonInfo{Mode: ""},
	})
	info, err := ctl.Info()
	require.NoError(t, err)
	require.Equal(t, "supervise", info.Mode)
	require.False(t, info.CanShutdown)
}

func TestSystemControlInfoInit(t *testing.T) {
	ctl := NewSystemControlWithPower(&fakePower{
		info: &miclient.DaemonInfo{Mode: "init"},
	})
	info, err := ctl.Info()
	require.NoError(t, err)
	require.Equal(t, "init", info.Mode)
	require.True(t, info.CanShutdown)
}

func TestSystemControlRequestShutdownEmptyModeNotInit(t *testing.T) {
	ctl := NewSystemControlWithPower(&fakePower{
		info: &miclient.DaemonInfo{Mode: ""},
	})
	err := ctl.RequestShutdown("poweroff")
	require.True(t, errors.Is(err, ErrSystemNotInit))
}

func TestSystemControlRequestShutdownSupervise(t *testing.T) {
	ctl := NewSystemControlWithPower(&fakePower{
		info: &miclient.DaemonInfo{Mode: "supervise"},
	})
	err := ctl.RequestShutdown("reboot")
	require.True(t, errors.Is(err, ErrSystemNotInit))
}

func TestSystemControlRequestShutdownInit(t *testing.T) {
	fp := &fakePower{info: &miclient.DaemonInfo{Mode: "init"}}
	ctl := NewSystemControlWithPower(fp)
	require.NoError(t, ctl.RequestShutdown("poweroff"))
	require.Equal(t, "poweroff", fp.lastMode)
}

func TestSystemControlNilPowerUnavailable(t *testing.T) {
	ctl := NewSystemControl(nil)
	_, err := ctl.Info()
	require.True(t, errors.Is(err, ErrSystemUnavailable))
	err = ctl.RequestShutdown("poweroff")
	require.True(t, errors.Is(err, ErrSystemUnavailable))
}

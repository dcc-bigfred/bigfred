package cmd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

type stubRadioStore struct{}

func (stubRadioStore) Append(_ context.Context, _ domain.RadioMessage, _ []string) (string, error) {
	return "1-0", nil
}

func (stubRadioStore) Replay(_ context.Context, _ []string, _ int) ([]domain.RadioMessage, error) {
	return nil, nil
}

func (stubRadioStore) ReplayLimit() int { return 200 }

type stubRadioLayouts map[uint]domain.Layout

func (s stubRadioLayouts) FindByID(_ context.Context, id uint) (domain.Layout, error) {
	layout, ok := s[id]
	if !ok {
		return domain.Layout{}, repo.ErrLayoutNotFound
	}
	return layout, nil
}

func TestRadioReplayRejectedWhenChatDisabled(t *testing.T) {
	ctx := context.Background()
	radio := cmd.NewRadio(cmd.RadioConfig{
		Store: stubRadioStore{},
		Layouts: stubRadioLayouts{
			1: {ID: 1, RadioChatEnabled: false},
		},
	})

	_, err := radio.ReplayUser(ctx, 1, 42, 10)
	if !errors.Is(err, svcerrors.ErrRadioChatDisabled) {
		t.Fatalf("expected ErrRadioChatDisabled, got %v", err)
	}
}

func TestRadioDeniedCodeMapsChatDisabled(t *testing.T) {
	if got := cmd.RadioDeniedCode(svcerrors.ErrRadioChatDisabled); got != "radio_chat_disabled" {
		t.Fatalf("expected radio_chat_disabled, got %q", got)
	}
}

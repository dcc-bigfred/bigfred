package protocol

import (
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// LeaseEntryResponse is one active lease row for REST list endpoints.
type LeaseEntryResponse struct {
	Kind       domain.TakeoverTarget `json:"kind"`
	TargetID   string                `json:"targetId"`
	TargetName string                `json:"targetName"`
	FromUserID uint                  `json:"fromUserId"`
	FromLogin  string                `json:"fromLogin"`
	ToUserID   uint                  `json:"toUserId"`
	ToLogin    string                `json:"toLogin"`
	ExpiresAt  time.Time             `json:"expiresAt"`
	SpeedLimit uint8                 `json:"speedLimit"`
}

// LendableTargetResponse is a vehicle or train available to lease.
type LendableTargetResponse struct {
	Kind       domain.TakeoverTarget `json:"kind"`
	TargetID   string                `json:"targetId"`
	TargetName string                `json:"targetName"`
}

// LendableUserResponse is a system account eligible as lessee.
type LendableUserResponse struct {
	UserID       uint   `json:"userId"`
	Login        string `json:"login"`
	Organization string `json:"organization,omitempty"`
}

// LendableResponse powers the create-lease dialog.
type LendableResponse struct {
	Targets []LendableTargetResponse `json:"targets"`
	Users   []LendableUserResponse   `json:"users"`
}

// CreateLeaseRequest is POST /api/v1/leases.
type CreateLeaseRequest struct {
	Kind             domain.TakeoverTarget `json:"kind"`
	TargetID         string                `json:"targetId"`
	ToUserID         uint                  `json:"toUserId"`
	SpeedLimit       uint8                 `json:"speedLimit"`
	DurationSeconds  int                   `json:"durationSeconds"`
}

// PatchLeaseRequest is PATCH /api/v1/leases/{kind}/{id}.
type PatchLeaseRequest struct {
	SpeedLimit      *uint8 `json:"speedLimit,omitempty"`
	DurationSeconds *int   `json:"durationSeconds,omitempty"`
}

func ToLeaseEntryResponse(e cmd.LeaseEntry) LeaseEntryResponse {
	return LeaseEntryResponse{
		Kind:       e.Kind,
		TargetID:   e.TargetID,
		TargetName: e.TargetName,
		FromUserID: e.FromUserID,
		FromLogin:  e.FromLogin,
		ToUserID:   e.ToUserID,
		ToLogin:    e.ToLogin,
		ExpiresAt:  e.ExpiresAt,
		SpeedLimit: e.SpeedLimit,
	}
}

func ToLendableResponse(c cmd.LendableCatalogue) LendableResponse {
	targets := make([]LendableTargetResponse, 0, len(c.Targets))
	for _, t := range c.Targets {
		targets = append(targets, LendableTargetResponse{
			Kind:       t.Kind,
			TargetID:   t.TargetID,
			TargetName: t.TargetName,
		})
	}
	users := make([]LendableUserResponse, 0, len(c.Users))
	for _, u := range c.Users {
		users = append(users, LendableUserResponse{
			UserID:       u.UserID,
			Login:        u.Login,
			Organization: u.Organization,
		})
	}
	return LendableResponse{Targets: targets, Users: users}
}

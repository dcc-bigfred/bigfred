package protocol

import (
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// DCCPoolRangeResponse is one user DCC pool range on the wire.
type DCCPoolRangeResponse struct {
	From uint16 `json:"from"`
	To   uint16 `json:"to"`
}

// DCCPoolRangeRequest is one user DCC pool range in create/update payloads.
type DCCPoolRangeRequest struct {
	From uint16 `json:"from"`
	To   uint16 `json:"to"`
}

// UserResponse is the JSON shape returned by user-management endpoints.
type UserResponse struct {
	ID           uint                   `json:"id"`
	Login        string                 `json:"login"`
	Organization string                 `json:"organization"`
	Role         domain.Role            `json:"role"`
	Active       bool                   `json:"active"`
	DCCPool      []DCCPoolRangeResponse `json:"dccPool"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// ToUserResponse maps a domain user and pool ranges to REST wire shape.
func ToUserResponse(u domain.User, pool []domain.DCCAddressRange) UserResponse {
	if pool == nil {
		pool = []domain.DCCAddressRange{}
	}
	return UserResponse{
		ID:           u.ID,
		Login:        u.Login,
		Organization: u.Organization,
		Role:         u.Role,
		Active:       u.Active,
		DCCPool:      ToDCCPoolRangeResponses(pool),
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// ToDCCPoolRangeResponses maps domain ranges to REST wire shape.
func ToDCCPoolRangeResponses(rows []domain.DCCAddressRange) []DCCPoolRangeResponse {
	out := make([]DCCPoolRangeResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, DCCPoolRangeResponse{From: r.FromAddr, To: r.ToAddr})
	}
	return out
}

// ToPoolRanges maps request ranges to cmd input.
func ToPoolRanges(rows []DCCPoolRangeRequest) []cmd.PoolRange {
	out := make([]cmd.PoolRange, 0, len(rows))
	for _, r := range rows {
		out = append(out, cmd.PoolRange{From: r.From, To: r.To})
	}
	return out
}

// UserCreateRequest is the POST /api/v1/users body.
type UserCreateRequest struct {
	Login        string                `json:"login"`
	PIN          string                `json:"pin"`
	Organization string                `json:"organization"`
	Role         domain.Role           `json:"role"`
	DCCPool      []DCCPoolRangeRequest `json:"dccPool"`
}

// ToCreateInput maps the HTTP body to cmd input.
func (r UserCreateRequest) ToCreateInput() cmd.UserCreateInput {
	return cmd.UserCreateInput{
		Login:        r.Login,
		PIN:          r.PIN,
		Organization: r.Organization,
		Role:         r.Role,
		DCCPool:      ToPoolRanges(r.DCCPool),
	}
}

// UserUpdateRequest mirrors optional fields exposed by cmd.User.Update.
type UserUpdateRequest struct {
	Login        *string                `json:"login"`
	Organization *string                `json:"organization"`
	Role         *domain.Role           `json:"role"`
	PIN          *string                `json:"pin"`
	DCCPool      *[]DCCPoolRangeRequest `json:"dccPool"`
}

// ToUpdateInput maps the HTTP body to cmd input.
func (r UserUpdateRequest) ToUpdateInput() cmd.UserUpdateInput {
	if r.PIN != nil && *r.PIN == "" {
		r.PIN = nil
	}
	in := cmd.UserUpdateInput{
		Login:        r.Login,
		Organization: r.Organization,
		Role:         r.Role,
		PIN:          r.PIN,
	}
	if r.DCCPool != nil {
		ranges := ToPoolRanges(*r.DCCPool)
		in.DCCPool = &ranges
	}
	return in
}

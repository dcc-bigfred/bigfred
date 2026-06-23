// Package httpapi resolves vehicle catalogue ids to DCC addresses via
// loco-server REST.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// Loco is one vehicle ready to drive on the data plane.
type Loco struct {
	VehicleID string
	Address   uint16
}

// Client calls authenticated loco-server REST endpoints.
type Client struct {
	apiBase string
	http    *http.Client
}

// NewClient returns a client that reuses the authenticated session HTTP
// client from login.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	apiBase, err := apiV1Base(baseURL)
	if err != nil {
		panic(err)
	}
	return &Client{apiBase: apiBase, http: httpClient}
}

// DiscoverDriveableVehicles resolves explicit vehicle ids, or when ids is
// empty loads every catalogue vehicle owned by userID that is on the active
// layout and has a DCC address.
func (c *Client) DiscoverDriveableVehicles(ctx context.Context, userID uint, vehicleIDs []string) ([]Loco, error) {
	if len(vehicleIDs) > 0 {
		return c.ResolveVehicles(ctx, vehicleIDs)
	}

	rows, err := c.fetchCatalogue(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Loco, 0)
	for _, row := range rows {
		if row.OwnerID != userID {
			continue
		}
		if row.DCCAddress == nil || row.IsDummy || !row.OnLayout {
			continue
		}
		out = append(out, Loco{VehicleID: row.ID, Address: *row.DCCAddress})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no driveable vehicles found for user on layout")
	}
	return out, nil
}

// ResolveVehicles maps vehicle ids (e.g. V-1) to DCC addresses using
// GET /api/v1/vehicles/catalogue.
func (c *Client) ResolveVehicles(ctx context.Context, vehicleIDs []string) ([]Loco, error) {
	rows, err := c.fetchCatalogue(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]protocol.VehicleCatalogueResponse, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	out := make([]Loco, 0, len(vehicleIDs))
	for _, id := range vehicleIDs {
		row, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("vehicle %q not found in catalogue", id)
		}
		if row.DCCAddress == nil {
			return nil, fmt.Errorf("vehicle %q has no DCC address", id)
		}
		if row.IsDummy {
			return nil, fmt.Errorf("vehicle %q is a dummy and cannot be driven", id)
		}
		if !row.OnLayout {
			return nil, fmt.Errorf("vehicle %q is not on the active layout roster", id)
		}
		out = append(out, Loco{VehicleID: id, Address: *row.DCCAddress})
	}
	return out, nil
}

func (c *Client) fetchCatalogue(ctx context.Context) ([]protocol.VehicleCatalogueResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/vehicles/catalogue", nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalogue request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("catalogue failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var rows []protocol.VehicleCatalogueResponse
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode catalogue: %w", err)
	}
	return rows, nil
}

func apiV1Base(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("http-addr is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse http-addr: %w", err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/v1"
	return strings.TrimSuffix(u.String(), "/"), nil
}

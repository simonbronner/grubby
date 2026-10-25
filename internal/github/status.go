package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var GitHubStatusURL = "https://www.githubstatus.com/api/v2/summary.json"

type StatusSummary struct {
	Page struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		URL       string    `json:"url"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"page"`
	Status struct {
		Indicator   string `json:"indicator"` // "none", "minor", "major", "critical"
		Description string `json:"description"`
	} `json:"status"`
	Components            []StatusComponent      `json:"components"`
	Incidents             []StatusIncident       `json:"incidents"`
	ScheduledMaintenances []ScheduledMaintenance `json:"scheduled_maintenances"`
}

type StatusComponent struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Status             string    `json:"status"` // "operational", "degraded_performance", "partial_outage", "major_outage"
	Description        string    `json:"description"`
	UpdatedAt          time.Time `json:"updated_at"`
	Position           int       `json:"position"`
	Showcase           bool      `json:"showcase"`
	GroupID            *string   `json:"group_id"`
	Group              bool      `json:"group"`
	OnlyShowIfDegraded bool      `json:"only_show_if_degraded"`
}

type StatusIncident struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Status          string           `json:"status"` // "investigating", "identified", "monitoring", "resolved"
	Impact          string           `json:"impact"` // "none", "minor", "major", "critical"
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Shortlink       string           `json:"shortlink"`
	IncidentUpdates []IncidentUpdate `json:"incident_updates"`
}

type IncidentUpdate struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type ScheduledMaintenance struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	ScheduledFor   time.Time `json:"scheduled_for"`
	ScheduledUntil time.Time `json:"scheduled_until"`
}

// FetchStatus retrieves the current service status from githubstatus.com
func (c *Client) FetchStatus(ctx context.Context) (*StatusSummary, error) {
	client := http.DefaultClient
	if c != nil && c.httpClient != nil {
		client = c.httpClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubStatusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create status request: %w", err)
	}
	req.Header.Set("User-Agent", "grubby-tui")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status response: %s", resp.Status)
	}

	var summary StatusSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return &summary, nil
}

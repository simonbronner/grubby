package storage

import (
	"strings"
	"time"

	"grubby/internal/github"
)

type RepoTier string

const (
	TierHot     RepoTier = "hot"
	TierActive  RepoTier = "active"
	TierDormant RepoTier = "dormant"
)

// CalculateRepoTier calculates the activity tier of a repository:
// 1. Hot:
//    - Interacted with recently (within last 7 days)
//    - Pushed to recently (within last 3 days)
//    - Has open pull requests
// 2. Active:
//    - Interacted with in the last 30 days
//    - Pushed to in the last 30 days
// 3. Dormant:
//    - Inactive for > 30 days with 0 open PRs
func CalculateRepoTier(r github.Repository, lastInteractedAt time.Time, openPRCount int) RepoTier {
	now := time.Now()

	// Hot conditions
	if !lastInteractedAt.IsZero() && now.Sub(lastInteractedAt) < 7*24*time.Hour {
		return TierHot
	}
	if !r.PushedAt.IsZero() && now.Sub(r.PushedAt) < 3*24*time.Hour {
		return TierHot
	}
	if openPRCount > 0 {
		return TierHot
	}

	// Active conditions
	if !lastInteractedAt.IsZero() && now.Sub(lastInteractedAt) < 30*24*time.Hour {
		return TierActive
	}
	lastUpdated := r.LastUpdated()
	if !lastUpdated.IsZero() && now.Sub(lastUpdated) < 30*24*time.Hour {
		return TierActive
	}

	return TierDormant
}

// TierBadgeText returns the text label for a tier.
func TierBadgeText(tier RepoTier) string {
	switch tier {
	case TierHot:
		return "🔥 HOT"
	case TierActive:
		return "⚡ ACTIVE"
	case TierDormant:
		return "💤 DORMANT"
	default:
		return "⚡ ACTIVE"
	}
}

// TierPriority returns sort priority (Hot = 0, Active = 1, Dormant = 2).
func TierPriority(tier RepoTier) int {
	switch strings.ToLower(string(tier)) {
	case "hot":
		return 0
	case "active":
		return 1
	case "dormant":
		return 2
	default:
		return 1
	}
}

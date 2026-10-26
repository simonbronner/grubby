package storage

import (
	"strings"
	"time"

	"grubby/internal/github"
)

type RepoTier string

const (
	TierHot  RepoTier = "hot"
	TierWarm RepoTier = "warm"
	TierCold RepoTier = "cold"
	TierDead RepoTier = "dead"

	// Backward compatibility aliases
	TierActive  RepoTier = "warm"
	TierDormant RepoTier = "cold"
)

const (
	HotThreshold  = 4 * 24 * time.Hour   // 4 days
	WarmThreshold = 42 * 24 * time.Hour  // 6 weeks (42 days)
	ColdThreshold = 365 * 24 * time.Hour // 12 months (365 days)

	// Backward compatibility alias
	ActiveThreshold = WarmThreshold
)

// IsDependabot returns true if the actor/author is Dependabot.
func IsDependabot(actor string) bool {
	a := strings.ToLower(strings.TrimSpace(actor))
	a = strings.TrimPrefix(a, "@")
	return a == "dependabot[bot]" || a == "dependabot" || a == "app/dependabot"
}

// TierIcon returns the visual emoji icon for a repository tier.
func TierIcon(tier RepoTier) string {
	switch strings.ToLower(string(tier)) {
	case "hot":
		return "🔥"
	case "warm", "active":
		return "⚡"
	case "cold", "dormant":
		return "❄️"
	case "dead":
		return "💀"
	default:
		return "⚡"
	}
}

// TierBadgeText returns the text label for a tier.
func TierBadgeText(tier RepoTier) string {
	switch strings.ToLower(string(tier)) {
	case "hot":
		return "🔥 HOT"
	case "warm", "active":
		return "⚡ WARM"
	case "cold", "dormant":
		return "❄️ COLD"
	case "dead":
		return "💀 DEAD"
	default:
		return "⚡ WARM"
	}
}

// TierPriority returns sort priority (Hot = 0, Warm = 1, Cold = 2, Dead = 3).
func TierPriority(tier RepoTier) int {
	switch strings.ToLower(string(tier)) {
	case "hot":
		return 0
	case "warm", "active":
		return 1
	case "cold", "dormant":
		return 2
	case "dead":
		return 3
	default:
		return 1
	}
}

// LatestActivityTime calculates the most recent non-dependabot activity timestamp
// for a repository across pushes, commits, and pull requests.
// Note: UI interaction in Grubby does not count as code activity and does not change tier.
func LatestActivityTime(r github.Repository, repoPRs []github.PullRequest, repoEvents []github.SquadEvent) time.Time {
	var latestActivity time.Time
	var latestDependabotActivity time.Time

	// 1. Pull requests
	for _, pr := range repoPRs {
		prTime := pr.UpdatedAt
		if prTime.IsZero() {
			prTime = pr.CreatedAt
		}
		if prTime.IsZero() {
			prTime = time.Now()
		}

		if IsDependabot(pr.Author) {
			if prTime.After(latestDependabotActivity) {
				latestDependabotActivity = prTime
			}
		} else {
			if prTime.After(latestActivity) {
				latestActivity = prTime
			}
		}
	}

	// 2. Commits and squad activity events
	for _, ev := range repoEvents {
		if IsDependabot(ev.Actor) {
			if ev.Timestamp.After(latestDependabotActivity) {
				latestDependabotActivity = ev.Timestamp
			}
		} else {
			if ev.Timestamp.After(latestActivity) {
				latestActivity = ev.Timestamp
			}
		}
	}

	// 3. Repository PushedAt
	if !r.PushedAt.IsZero() {
		isDependabotPush := false
		if !latestDependabotActivity.IsZero() {
			diff := r.PushedAt.Sub(latestDependabotActivity)
			if diff >= -5*time.Minute && diff <= 5*time.Minute {
				isDependabotPush = true
			}
		}
		if !isDependabotPush && r.PushedAt.After(latestActivity) {
			latestActivity = r.PushedAt
		}
	}

	return latestActivity
}

// CalculateRepoTier calculates the activity tier of a repository:
// - Hot: activity within 4 days (not @dependabot[bot])
// - Warm: activity between 4 days and 6 weeks (42 days)
// - Cold: activity between 6 weeks and 12 months (365 days)
// - Dead: activity over 12 months (> 365 days or zero activity)
func CalculateRepoTier(r github.Repository, repoPRs []github.PullRequest, repoEvents []github.SquadEvent) RepoTier {
	latest := LatestActivityTime(r, repoPRs, repoEvents)
	if latest.IsZero() {
		return TierDead
	}
	age := time.Since(latest)
	if age < 0 {
		age = 0
	}
	if age <= HotThreshold {
		return TierHot
	}
	if age <= WarmThreshold {
		return TierWarm
	}
	if age <= ColdThreshold {
		return TierCold
	}
	return TierDead
}

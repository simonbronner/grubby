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

const (
	HotThreshold    = 4 * 24 * time.Hour  // 4 days
	ActiveThreshold = 42 * 24 * time.Hour // 6 weeks (42 days)
)

// IsDependabot returns true if the actor/author is Dependabot.
func IsDependabot(actor string) bool {
	a := strings.ToLower(strings.TrimSpace(actor))
	a = strings.TrimPrefix(a, "@")
	return a == "dependabot[bot]" || a == "dependabot" || a == "app/dependabot"
}

// TierIcon returns the visual emoji icon for a repository tier.
func TierIcon(tier RepoTier) string {
	switch tier {
	case TierHot:
		return "🔥"
	case TierActive:
		return "⚡"
	case TierDormant:
		return "💤"
	default:
		return "⚡"
	}
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

// LatestActivityTime calculates the most recent non-dependabot activity timestamp
// for a repository across pushes, commits, pull requests, and human interaction.
func LatestActivityTime(r github.Repository, lastInteractedAt time.Time, repoPRs []github.PullRequest, repoEvents []github.SquadEvent) time.Time {
	var latestActivity time.Time
	var latestDependabotActivity time.Time

	// 1. Human user interaction with repository in Grubby
	if !lastInteractedAt.IsZero() && lastInteractedAt.After(latestActivity) {
		latestActivity = lastInteractedAt
	}

	// 2. Pull requests
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

	// 3. Commits and squad activity events
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

	// 4. Repository PushedAt
	if !r.PushedAt.IsZero() {
		isDependabotPush := false
		if !latestDependabotActivity.IsZero() {
			diff := r.PushedAt.Sub(latestDependabotActivity)
			if diff >= -5*time.Minute && diff <= 5*time.Minute {
				isDependabotPush = true
			}
		}
		if !isDependabotPush && r.PushedAt.After(latestDependabotActivity) && r.PushedAt.After(latestActivity) {
			latestActivity = r.PushedAt
		}
	}

	return latestActivity
}

// CalculateRepoTier calculates the activity tier of a repository:
// - Hot: activity within 4 days (not @dependabot[bot])
// - Active: activity between 4 days and 6 weeks (42 days)
// - Dormant: beyond 6 weeks
func CalculateRepoTier(r github.Repository, lastInteractedAt time.Time, repoPRs []github.PullRequest, repoEvents []github.SquadEvent) RepoTier {
	latest := LatestActivityTime(r, lastInteractedAt, repoPRs, repoEvents)
	if latest.IsZero() {
		return TierDormant
	}
	age := time.Since(latest)
	if age < 0 {
		age = 0
	}
	if age <= HotThreshold {
		return TierHot
	}
	if age <= ActiveThreshold {
		return TierActive
	}
	return TierDormant
}

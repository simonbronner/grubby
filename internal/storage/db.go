package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"grubby/internal/github"
)

// DefaultDBPath returns ~/.config/grubby/grubby.db
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "grubby", "grubby.db"), nil
}

type DB struct {
	db *sql.DB
	mu sync.RWMutex
}

// Open initializes the SQLite database with WAL mode and creates required tables.
func Open(dbPath string) (*DB, error) {
	if dbPath == "" {
		var err error
		dbPath, err = DefaultDBPath()
		if err != nil {
			return nil, fmt.Errorf("failed to determine default db path: %w", err)
		}
	}

	if dbPath != ":memory:" && !strings.Contains(dbPath, "mode=memory") {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory %q: %w", dir, err)
		}
	}

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure connection pool for SQLite
	sqlDB.SetMaxOpenConns(1) // modernc sqlite performs best with serialized writer
	sqlDB.SetMaxIdleConns(1)

	// Set pragmas for performance and concurrency
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := sqlDB.Exec(p); err != nil {
			// Some pragmas might fail on in-memory db, ignore harmless errors
			_ = err
		}
	}

	db := &DB{db: sqlDB}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS repositories (
		name TEXT PRIMARY KEY,
		full_name TEXT NOT NULL,
		owner TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		pushed_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		default_branch TEXT NOT NULL DEFAULT 'main',
		is_private INTEGER NOT NULL DEFAULT 0,
		url TEXT NOT NULL DEFAULT '',
		ssh_url TEXT NOT NULL DEFAULT '',
		last_interacted_at DATETIME,
		tier TEXT NOT NULL DEFAULT 'warm',
		last_synced_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS pull_requests (
		repo TEXT NOT NULL,
		number INTEGER NOT NULL,
		title TEXT NOT NULL,
		author TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		state TEXT NOT NULL DEFAULT 'OPEN',
		is_draft INTEGER NOT NULL DEFAULT 0,
		review_decision TEXT NOT NULL DEFAULT '',
		labels_json TEXT NOT NULL DEFAULT '[]',
		total_comments INTEGER NOT NULL DEFAULT 0,
		unresolved_comments INTEGER NOT NULL DEFAULT 0,
		url TEXT NOT NULL DEFAULT '',
		head_ref_name TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (repo, number)
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY,
		number INTEGER NOT NULL,
		repo_owner TEXT NOT NULL,
		repo_name TEXT NOT NULL,
		title TEXT NOT NULL,
		body TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL DEFAULT 'open',
		priority TEXT NOT NULL DEFAULT 'medium',
		author TEXT NOT NULL DEFAULT '',
		assignees_json TEXT NOT NULL DEFAULT '[]',
		labels_json TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		closed_at DATETIME,
		url TEXT NOT NULL DEFAULT '',
		is_squad_task INTEGER NOT NULL DEFAULT 0,
		sync_status TEXT NOT NULL DEFAULT 'synced'
	);
	CREATE INDEX IF NOT EXISTS idx_tasks_lookup ON tasks (repo_owner, repo_name, number);
	CREATE INDEX IF NOT EXISTS idx_tasks_squad_state ON tasks (is_squad_task, state);

	CREATE TABLE IF NOT EXISTS squad_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		actor TEXT NOT NULL,
		action TEXT NOT NULL,
		repo TEXT NOT NULL,
		target TEXT NOT NULL,
		details TEXT NOT NULL DEFAULT '',
		timestamp DATETIME NOT NULL,
		url TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_squad_activity_timestamp ON squad_activity (timestamp DESC);

	CREATE TABLE IF NOT EXISTS org_members (
		login TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		avatar_url TEXT NOT NULL DEFAULT '',
		id TEXT NOT NULL DEFAULT '',
		database_id INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS security_alerts (
		repo TEXT PRIMARY KEY,
		critical INTEGER NOT NULL DEFAULT 0,
		high INTEGER NOT NULL DEFAULT 0,
		medium INTEGER NOT NULL DEFAULT 0,
		low INTEGER NOT NULL DEFAULT 0,
		total INTEGER NOT NULL DEFAULT 0,
		packages_json TEXT NOT NULL DEFAULT '[]',
		last_scanned DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS app_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS repo_labels (
		repo_id INTEGER NOT NULL,
		repo_name TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL,
		squad TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (repo_id, label, squad)
	);
	CREATE INDEX IF NOT EXISTS idx_repo_labels_squad ON repo_labels (squad);
	CREATE INDEX IF NOT EXISTS idx_repo_labels_name ON repo_labels (repo_name);

	CREATE TABLE IF NOT EXISTS label_meta (
		squad TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL,
		color TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (squad, label)
	);
	CREATE INDEX IF NOT EXISTS idx_label_meta_squad ON label_meta (squad);
	`
	_, err := d.db.Exec(schema)
	return err
}

// --- Repositories ---

func (d *DB) SaveRepositories(repos []github.Repository, owner string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO repositories (
			name, full_name, owner, description, pushed_at, updated_at,
			default_branch, is_private, url, ssh_url, last_synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			full_name=excluded.full_name,
			owner=excluded.owner,
			description=excluded.description,
			pushed_at=excluded.pushed_at,
			updated_at=excluded.updated_at,
			default_branch=excluded.default_branch,
			is_private=excluded.is_private,
			url=excluded.url,
			ssh_url=excluded.ssh_url,
			last_synced_at=excluded.last_synced_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, r := range repos {
		isPriv := 0
		if r.IsPrivate {
			isPriv = 1
		}
		repoOwner := owner
		if repoOwner == "" {
			parts := strings.Split(r.FullName, "/")
			if len(parts) > 0 {
				repoOwner = parts[0]
			}
		}

		_, err := stmt.Exec(
			r.Name, r.FullName, repoOwner, r.Description,
			r.PushedAt.Format(time.RFC3339), r.UpdatedAt.Format(time.RFC3339),
			r.DefaultBranch, isPriv, r.URL, r.SSHURL, now.Format(time.RFC3339),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) GetRepositories() ([]github.Repository, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT name, full_name, description, pushed_at, updated_at, default_branch, is_private, url, ssh_url
		FROM repositories
		ORDER BY pushed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []github.Repository
	for rows.Next() {
		var r github.Repository
		var pushedStr, updatedStr string
		var isPriv int

		err := rows.Scan(
			&r.Name, &r.FullName, &r.Description,
			&pushedStr, &updatedStr, &r.DefaultBranch,
			&isPriv, &r.URL, &r.SSHURL,
		)
		if err != nil {
			return nil, err
		}
		r.IsPrivate = isPriv != 0
		r.PushedAt, _ = time.Parse(time.RFC3339, pushedStr)
		r.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		repos = append(repos, r)
	}

	return repos, nil
}

func (d *DB) TouchRepo(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Format(time.RFC3339)
	_, err := d.db.Exec(`
		UPDATE repositories
		SET last_interacted_at = ?
		WHERE name = ? OR LOWER(name) = LOWER(?)
	`, now, name, name)
	return err
}

func (d *DB) GetRepoLastInteractedAt(name string) time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var tStr sql.NullString
	err := d.db.QueryRow(`
		SELECT last_interacted_at FROM repositories WHERE name = ? OR LOWER(name) = LOWER(?)
	`, name, name).Scan(&tStr)
	if err != nil || !tStr.Valid || tStr.String == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, tStr.String)
	return t
}

func (d *DB) GetAllRepoInteractions() map[string]time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()

	res := make(map[string]time.Time)
	rows, err := d.db.Query(`SELECT name, last_interacted_at FROM repositories WHERE last_interacted_at IS NOT NULL`)
	if err != nil {
		return res
	}
	defer rows.Close()

	for rows.Next() {
		var name, tStr string
		if err := rows.Scan(&name, &tStr); err == nil && tStr != "" {
			if t, err := time.Parse(time.RFC3339, tStr); err == nil {
				res[strings.ToLower(name)] = t
			}
		}
	}
	return res
}

func (d *DB) SetRepoTier(name string, tier RepoTier) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE repositories SET tier = ? WHERE name = ? OR LOWER(name) = LOWER(?)`, string(tier), name, name)
	return err
}

func (d *DB) GetRepoTier(name string) RepoTier {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var tierStr string
	err := d.db.QueryRow(`SELECT tier FROM repositories WHERE name = ? OR LOWER(name) = LOWER(?)`, name, name).Scan(&tierStr)
	if err != nil || tierStr == "" {
		return TierWarm
	}
	switch strings.ToLower(tierStr) {
	case "hot":
		return TierHot
	case "warm", "active":
		return TierWarm
	case "cold", "dormant":
		return TierCold
	case "dead":
		return TierDead
	default:
		return RepoTier(tierStr)
	}
}

func (d *DB) GetAllRepoTiers() map[string]RepoTier {
	d.mu.RLock()
	defer d.mu.RUnlock()

	res := make(map[string]RepoTier)
	rows, err := d.db.Query(`SELECT name, tier FROM repositories`)
	if err != nil {
		return res
	}
	defer rows.Close()

	for rows.Next() {
		var name, tier string
		if err := rows.Scan(&name, &tier); err == nil && tier != "" {
			switch strings.ToLower(tier) {
			case "hot":
				res[strings.ToLower(name)] = TierHot
			case "warm", "active":
				res[strings.ToLower(name)] = TierWarm
			case "cold", "dormant":
				res[strings.ToLower(name)] = TierCold
			case "dead":
				res[strings.ToLower(name)] = TierDead
			default:
				res[strings.ToLower(name)] = RepoTier(tier)
			}
		}
	}
	return res
}

func (d *DB) GetHotAndWarmRepos() ([]github.Repository, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT name, full_name, description, pushed_at, updated_at, default_branch, is_private, url, ssh_url
		FROM repositories
		WHERE tier IN ('hot', 'warm', 'active')
		ORDER BY pushed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []github.Repository
	for rows.Next() {
		var r github.Repository
		var pushedStr, updatedStr string
		var isPriv int

		err := rows.Scan(
			&r.Name, &r.FullName, &r.Description,
			&pushedStr, &updatedStr, &r.DefaultBranch,
			&isPriv, &r.URL, &r.SSHURL,
		)
		if err != nil {
			return nil, err
		}
		r.IsPrivate = isPriv != 0
		r.PushedAt, _ = time.Parse(time.RFC3339, pushedStr)
		r.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		repos = append(repos, r)
	}

	return repos, nil
}

func (d *DB) GetHotAndActiveRepos() ([]github.Repository, error) {
	return d.GetHotAndWarmRepos()
}

func (d *DB) GetDormantRepos() ([]github.Repository, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT name, full_name, description, pushed_at, updated_at, default_branch, is_private, url, ssh_url
		FROM repositories
		WHERE tier = 'dormant'
		ORDER BY pushed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []github.Repository
	for rows.Next() {
		var r github.Repository
		var pushedStr, updatedStr string
		var isPriv int

		err := rows.Scan(
			&r.Name, &r.FullName, &r.Description,
			&pushedStr, &updatedStr, &r.DefaultBranch,
			&isPriv, &r.URL, &r.SSHURL,
		)
		if err != nil {
			return nil, err
		}
		r.IsPrivate = isPriv != 0
		r.PushedAt, _ = time.Parse(time.RFC3339, pushedStr)
		r.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		repos = append(repos, r)
	}

	return repos, nil
}

func (d *DB) GetTierCounts() (hot, warm, cold, dead int, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT tier, COUNT(*) FROM repositories GROUP BY tier`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var tier string
		var count int
		if err := rows.Scan(&tier, &count); err == nil {
			switch strings.ToLower(tier) {
			case "hot":
				hot = count
			case "warm", "active":
				warm += count
			case "cold", "dormant":
				cold += count
			case "dead":
				dead = count
			}
		}
	}
	return hot, warm, cold, dead, nil
}

// UpdateRepoTiers computes and updates the tier for each repository based on activity
func (d *DB) UpdateRepoTiers(prs []github.PullRequest, events ...[]github.SquadEvent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	repoPRs := make(map[string][]github.PullRequest)
	for _, pr := range prs {
		norm := strings.ToLower(github.NormalizeRepoName(pr.Repo))
		repoPRs[norm] = append(repoPRs[norm], pr)
	}

	repoEvents := make(map[string][]github.SquadEvent)
	if len(events) > 0 && len(events[0]) > 0 {
		for _, ev := range events[0] {
			norm := strings.ToLower(github.NormalizeRepoName(ev.Repo))
			repoEvents[norm] = append(repoEvents[norm], ev)
		}
	} else {
		// Load recent squad events from database if available
		squadRows, err := d.db.Query(`SELECT actor, action, repo, target, details, timestamp, url FROM squad_activity ORDER BY timestamp DESC LIMIT 500`)
		if err == nil {
			defer squadRows.Close()
			for squadRows.Next() {
				var e github.SquadEvent
				var tStr string
				if err := squadRows.Scan(&e.Actor, &e.Action, &e.Repo, &e.Target, &e.Details, &tStr, &e.URL); err == nil {
					e.Timestamp, _ = time.Parse(time.RFC3339, tStr)
					norm := strings.ToLower(github.NormalizeRepoName(e.Repo))
					repoEvents[norm] = append(repoEvents[norm], e)
				}
			}
		}
	}

	rows, err := d.db.Query(`SELECT name, full_name, description, pushed_at, updated_at, default_branch, is_private, url, ssh_url, last_interacted_at FROM repositories`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type repoWithMeta struct {
		repo           github.Repository
		lastInteracted time.Time
	}
	var list []repoWithMeta

	for rows.Next() {
		var r github.Repository
		var pushedStr, updatedStr string
		var isPriv int
		var interactStr sql.NullString

		if err := rows.Scan(
			&r.Name, &r.FullName, &r.Description,
			&pushedStr, &updatedStr, &r.DefaultBranch,
			&isPriv, &r.URL, &r.SSHURL, &interactStr,
		); err != nil {
			continue
		}
		r.IsPrivate = isPriv != 0
		r.PushedAt, _ = time.Parse(time.RFC3339, pushedStr)
		r.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		var lastInt time.Time
		if interactStr.Valid && interactStr.String != "" {
			lastInt, _ = time.Parse(time.RFC3339, interactStr.String)
		}
		list = append(list, repoWithMeta{repo: r, lastInteracted: lastInt})
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE repositories SET tier = ? WHERE name = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range list {
		norm := strings.ToLower(github.NormalizeRepoName(item.repo.Name))
		tier := CalculateRepoTier(item.repo, repoPRs[norm], repoEvents[norm])
		if _, err := stmt.Exec(string(tier), item.repo.Name); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// --- Pull Requests ---

func (d *DB) SavePullRequests(prs []github.PullRequest) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear previous PR cache
	if _, err := tx.Exec(`DELETE FROM pull_requests`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO pull_requests (
			repo, number, title, author, created_at, updated_at, state,
			is_draft, review_decision, labels_json, total_comments,
			unresolved_comments, url, head_ref_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pr := range prs {
		labelsJSON, _ := json.Marshal(pr.Labels)
		isDraft := 0
		if pr.IsDraft {
			isDraft = 1
		}
		state := pr.State
		if state == "" {
			state = "OPEN"
		}

		_, err := stmt.Exec(
			pr.Repo, pr.Number, pr.Title, pr.Author,
			pr.CreatedAt.Format(time.RFC3339), pr.UpdatedAt.Format(time.RFC3339),
			state, isDraft, pr.ReviewDecision, string(labelsJSON),
			pr.TotalComments, pr.UnresolvedComments, pr.URL, pr.HeadRefName,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) GetPullRequests() ([]github.PullRequest, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT repo, number, title, author, created_at, updated_at, state,
		       is_draft, review_decision, labels_json, total_comments,
		       unresolved_comments, url, head_ref_name
		FROM pull_requests
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []github.PullRequest
	for rows.Next() {
		var pr github.PullRequest
		var createdStr, updatedStr, labelsJSON string
		var isDraft int

		err := rows.Scan(
			&pr.Repo, &pr.Number, &pr.Title, &pr.Author,
			&createdStr, &updatedStr, &pr.State,
			&isDraft, &pr.ReviewDecision, &labelsJSON,
			&pr.TotalComments, &pr.UnresolvedComments,
			&pr.URL, &pr.HeadRefName,
		)
		if err != nil {
			return nil, err
		}
		pr.IsDraft = isDraft != 0
		pr.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		pr.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		_ = json.Unmarshal([]byte(labelsJSON), &pr.Labels)
		prs = append(prs, pr)
	}

	return prs, nil
}

func (d *DB) DeletePullRequest(repo string, number int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	norm := github.NormalizeRepoName(repo)
	_, err := d.db.Exec(`
		DELETE FROM pull_requests
		WHERE (repo = ? OR LOWER(repo) = LOWER(?) OR repo = ? OR LOWER(repo) = LOWER(?))
		  AND number = ?
	`, repo, repo, norm, norm, number)
	return err
}

// --- Tasks ---

func (d *DB) SaveTask(task *github.Task, syncStatus string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if syncStatus == "" {
		syncStatus = "synced"
	}

	assigneesJSON, _ := json.Marshal(task.Assignees)
	labelsJSON, _ := json.Marshal(task.Labels)
	isSquad := 0
	if task.IsSquadTask {
		isSquad = 1
	}

	var closedStr sql.NullString
	if task.ClosedAt != nil && !task.ClosedAt.IsZero() {
		closedStr = sql.NullString{String: task.ClosedAt.Format(time.RFC3339), Valid: true}
	}

	var id int64
	if task.ID == 0 {
		// Local temporary ID: generate negative timestamp to avoid collision with GitHub issue IDs
		id = -time.Now().UnixNano()
		task.ID = id
	} else {
		id = task.ID
	}

	_, err := d.db.Exec(`
		INSERT INTO tasks (
			id, number, repo_owner, repo_name, title, body, state, priority,
			author, assignees_json, labels_json, created_at, updated_at,
			closed_at, url, is_squad_task, sync_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			number=excluded.number,
			repo_owner=excluded.repo_owner,
			repo_name=excluded.repo_name,
			title=excluded.title,
			body=excluded.body,
			state=excluded.state,
			priority=excluded.priority,
			author=excluded.author,
			assignees_json=excluded.assignees_json,
			labels_json=excluded.labels_json,
			created_at=excluded.created_at,
			updated_at=excluded.updated_at,
			closed_at=excluded.closed_at,
			url=excluded.url,
			is_squad_task=excluded.is_squad_task,
			sync_status=excluded.sync_status
	`, id, task.Number, task.RepoOwner, task.RepoName, task.Title, task.Body,
		task.State, string(task.Priority), task.Author, string(assigneesJSON),
		string(labelsJSON), task.CreatedAt.Format(time.RFC3339),
		task.UpdatedAt.Format(time.RFC3339), closedStr, task.URL, isSquad, syncStatus)

	return err
}

func (d *DB) SaveTasks(tasks []github.Task, isSquad bool, syncStatus string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if syncStatus == "" {
		syncStatus = "synced"
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO tasks (
			id, number, repo_owner, repo_name, title, body, state, priority,
			author, assignees_json, labels_json, created_at, updated_at,
			closed_at, url, is_squad_task, sync_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			number=excluded.number,
			repo_owner=excluded.repo_owner,
			repo_name=excluded.repo_name,
			title=excluded.title,
			body=excluded.body,
			state=excluded.state,
			priority=excluded.priority,
			author=excluded.author,
			assignees_json=excluded.assignees_json,
			labels_json=excluded.labels_json,
			created_at=excluded.created_at,
			updated_at=excluded.updated_at,
			closed_at=excluded.closed_at,
			url=excluded.url,
			is_squad_task=excluded.is_squad_task,
			sync_status=excluded.sync_status
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, task := range tasks {
		assigneesJSON, _ := json.Marshal(task.Assignees)
		labelsJSON, _ := json.Marshal(task.Labels)
		isSquadInt := 0
		if isSquad || task.IsSquadTask {
			isSquadInt = 1
		}

		var closedStr sql.NullString
		if task.ClosedAt != nil && !task.ClosedAt.IsZero() {
			closedStr = sql.NullString{String: task.ClosedAt.Format(time.RFC3339), Valid: true}
		}

		id := task.ID
		if id == 0 {
			id = -time.Now().UnixNano()
		}

		_, err := stmt.Exec(
			id, task.Number, task.RepoOwner, task.RepoName, task.Title, task.Body,
			task.State, string(task.Priority), task.Author, string(assigneesJSON),
			string(labelsJSON), task.CreatedAt.Format(time.RFC3339),
			task.UpdatedAt.Format(time.RFC3339), closedStr, task.URL, isSquadInt, syncStatus,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) GetTasks(isSquad bool, state string) ([]github.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	isSquadInt := 0
	if isSquad {
		isSquadInt = 1
	}

	query := `
		SELECT id, number, repo_owner, repo_name, title, body, state, priority,
		       author, assignees_json, labels_json, created_at, updated_at,
		       closed_at, url, is_squad_task
		FROM tasks
		WHERE is_squad_task = ?
	`
	args := []interface{}{isSquadInt}

	if state != "" {
		query += " AND state = ?"
		args = append(args, state)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []github.Task
	for rows.Next() {
		var t github.Task
		var createdStr, updatedStr, assigneesJSON, labelsJSON, prioStr string
		var closedStr sql.NullString
		var isSquadVal int

		err := rows.Scan(
			&t.ID, &t.Number, &t.RepoOwner, &t.RepoName, &t.Title, &t.Body,
			&t.State, &prioStr, &t.Author, &assigneesJSON, &labelsJSON,
			&createdStr, &updatedStr, &closedStr, &t.URL, &isSquadVal,
		)
		if err != nil {
			return nil, err
		}
		t.Priority = github.TaskPriority(prioStr)
		t.IsSquadTask = isSquadVal != 0
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		if closedStr.Valid && closedStr.String != "" {
			cl, _ := time.Parse(time.RFC3339, closedStr.String)
			t.ClosedAt = &cl
		}
		_ = json.Unmarshal([]byte(assigneesJSON), &t.Assignees)
		_ = json.Unmarshal([]byte(labelsJSON), &t.Labels)
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (d *DB) DeleteTask(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (d *DB) UpdateTaskSyncStatus(id int64, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE tasks SET sync_status = ? WHERE id = ?`, status, id)
	return err
}

// --- Squad Activity ---

func (d *DB) SaveSquadEvents(events []github.SquadEvent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear old events to keep current window
	if _, err := tx.Exec(`DELETE FROM squad_activity`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO squad_activity (actor, action, repo, target, details, timestamp, url)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		_, err := stmt.Exec(
			e.Actor, e.Action, e.Repo, e.Target, e.Details,
			e.Timestamp.Format(time.RFC3339), e.URL,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) GetSquadEvents(limit int) ([]github.SquadEvent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := d.db.Query(`
		SELECT actor, action, repo, target, details, timestamp, url
		FROM squad_activity
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []github.SquadEvent
	for rows.Next() {
		var e github.SquadEvent
		var tStr string

		if err := rows.Scan(&e.Actor, &e.Action, &e.Repo, &e.Target, &e.Details, &tStr, &e.URL); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, tStr)
		events = append(events, e)
	}

	return events, nil
}

// --- Org Members ---

func (d *DB) SaveOrgMembers(members []github.OrgMember) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO org_members (login, name, avatar_url, id, database_id)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(login) DO UPDATE SET
			name=CASE WHEN excluded.name != '' THEN excluded.name ELSE org_members.name END,
			avatar_url=CASE WHEN excluded.avatar_url != '' THEN excluded.avatar_url ELSE org_members.avatar_url END,
			id=CASE WHEN excluded.id != '' THEN excluded.id ELSE org_members.id END,
			database_id=CASE WHEN excluded.database_id > 0 THEN excluded.database_id ELSE org_members.database_id END
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range members {
		if strings.TrimSpace(m.Login) == "" {
			continue
		}
		_, err := stmt.Exec(m.Login, m.Name, m.AvatarURL, m.ID, m.DatabaseID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) GetOrgMembers() ([]github.OrgMember, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT login, name, avatar_url, id, database_id FROM org_members ORDER BY login ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []github.OrgMember
	for rows.Next() {
		var m github.OrgMember
		if err := rows.Scan(&m.Login, &m.Name, &m.AvatarURL, &m.ID, &m.DatabaseID); err != nil {
			return nil, err
		}
		members = append(members, m)
	}

	return members, nil
}

// --- Security Alerts ---

func (d *DB) SaveSecuritySummaries(summaries map[string]github.SecurityAlertSummary) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO security_alerts (repo, critical, high, medium, low, total, packages_json, last_scanned)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo) DO UPDATE SET
			critical=excluded.critical,
			high=excluded.high,
			medium=excluded.medium,
			low=excluded.low,
			total=excluded.total,
			packages_json=excluded.packages_json,
			last_scanned=excluded.last_scanned
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for repo, s := range summaries {
		pkgsJSON, _ := json.Marshal(s.Packages)
		lastScanned := s.LastScanned
		if lastScanned.IsZero() {
			lastScanned = time.Now()
		}
		_, err := stmt.Exec(repo, s.Critical, s.High, s.Medium, s.Low, s.Total, string(pkgsJSON), lastScanned.Format(time.RFC3339))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) GetSecuritySummaries() (map[string]github.SecurityAlertSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT repo, critical, high, medium, low, total, packages_json, last_scanned FROM security_alerts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]github.SecurityAlertSummary)
	for rows.Next() {
		var s github.SecurityAlertSummary
		var pkgsJSON, lastScannedStr string

		if err := rows.Scan(&s.Repo, &s.Critical, &s.High, &s.Medium, &s.Low, &s.Total, &pkgsJSON, &lastScannedStr); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(pkgsJSON), &s.Packages)
		s.LastScanned, _ = time.Parse(time.RFC3339, lastScannedStr)
		res[s.Repo] = s
	}

	return res, nil
}

// --- App Meta & Request Statistics ---

func (d *DB) GetMeta(key string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var val string
	err := d.db.QueryRow(`SELECT value FROM app_meta WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (d *DB) SetMeta(key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO app_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (d *DB) GetRequestStats() (made int64, saved int64, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var madeStr, savedStr string
	_ = d.db.QueryRow(`SELECT value FROM app_meta WHERE key = 'requests_made'`).Scan(&madeStr)
	_ = d.db.QueryRow(`SELECT value FROM app_meta WHERE key = 'requests_saved'`).Scan(&savedStr)

	made, _ = strconv.ParseInt(madeStr, 10, 64)
	saved, _ = strconv.ParseInt(savedStr, 10, 64)
	return made, saved, nil
}

func (d *DB) IncrementRequestsMade(delta int64) error {
	if delta <= 0 {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	var madeStr string
	_ = d.db.QueryRow(`SELECT value FROM app_meta WHERE key = 'requests_made'`).Scan(&madeStr)
	current, _ := strconv.ParseInt(madeStr, 10, 64)
	current += delta

	_, err := d.db.Exec(`
		INSERT INTO app_meta (key, value) VALUES ('requests_made', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, strconv.FormatInt(current, 10))
	return err
}

func (d *DB) IncrementRequestsSaved(delta int64) error {
	if delta <= 0 {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	var savedStr string
	_ = d.db.QueryRow(`SELECT value FROM app_meta WHERE key = 'requests_saved'`).Scan(&savedStr)
	current, _ := strconv.ParseInt(savedStr, 10, 64)
	current += delta

	_, err := d.db.Exec(`
		INSERT INTO app_meta (key, value) VALUES ('requests_saved', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, strconv.FormatInt(current, 10))
	return err
}

// --- Repo Labels ---

func (d *DB) SaveRepoLabels(repoID int64, repoName string, labels []string, squad string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing labels for this repo in this squad
	if repoID > 0 {
		_, err = tx.Exec(`DELETE FROM repo_labels WHERE repo_id = ? AND squad = ?`, repoID, squad)
	} else if repoName != "" {
		_, err = tx.Exec(`DELETE FROM repo_labels WHERE repo_name = ? AND squad = ?`, repoName, squad)
	}
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO repo_labels (repo_id, repo_name, label, squad, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, label, squad) DO UPDATE SET
			repo_name = excluded.repo_name,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, l := range labels {
		clean := strings.ToLower(strings.TrimSpace(l))
		if clean == "" {
			continue
		}
		_, err = stmt.Exec(repoID, repoName, clean, squad, now)
		if err != nil {
			return err
		}
	}

	// Garbage collect any labels from label_meta that have no repositories in repo_labels for this squad
	_, _ = tx.Exec(`
		DELETE FROM label_meta
		WHERE squad = ? AND label NOT IN (
			SELECT DISTINCT label FROM repo_labels WHERE squad = ?
		)
	`, squad, squad)

	return tx.Commit()
}

func (d *DB) SaveLabelColors(squad string, colors map[string]string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO label_meta (squad, label, color)
		VALUES (?, ?, ?)
		ON CONFLICT(squad, label) DO UPDATE SET
			color = excluded.color
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for l, c := range colors {
		clean := strings.ToLower(strings.TrimSpace(l))
		if clean == "" {
			continue
		}
		if _, err := stmt.Exec(squad, clean, c); err != nil {
			return err
		}
	}

	// Garbage collect any labels that have no repositories in repo_labels for this squad
	_, _ = tx.Exec(`
		DELETE FROM label_meta
		WHERE squad = ? AND label NOT IN (
			SELECT DISTINCT label FROM repo_labels WHERE squad = ?
		)
	`, squad, squad)

	return tx.Commit()
}

func (d *DB) GetLabelColors(squad string) (map[string]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT label, color FROM label_meta WHERE squad = ?`, squad)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var l, c string
		if err := rows.Scan(&l, &c); err != nil {
			return nil, err
		}
		result[l] = c
	}
	return result, rows.Err()
}

func (d *DB) GetRepoLabelsForSquad(squad string) (map[int64][]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT repo_id, label FROM repo_labels WHERE squad = ? ORDER BY label ASC`, squad)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]string)
	for rows.Next() {
		var repoID int64
		var label string
		if err := rows.Scan(&repoID, &label); err != nil {
			return nil, err
		}
		result[repoID] = append(result[repoID], label)
	}
	return result, rows.Err()
}

func (d *DB) GetRepoLabelsByNameForSquad(squad string) (map[string][]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT repo_name, label FROM repo_labels WHERE squad = ? AND repo_name != '' ORDER BY label ASC`, squad)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var repoName string
		var label string
		if err := rows.Scan(&repoName, &label); err != nil {
			return nil, err
		}
		result[repoName] = append(result[repoName], label)
	}
	return result, rows.Err()
}

func (d *DB) GetAllUniqueLabels(squad string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT DISTINCT label FROM repo_labels WHERE squad = ? ORDER BY label ASC`, squad)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

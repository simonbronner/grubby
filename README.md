# Grubby

[![Go Report Card](https://goreportcard.com/badge/github.com/simonbronner/grubby)](https://goreportcard.com/report/github.com/simonbronner/grubby)
[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Grubby** is a high-performance Terminal User Interface (TUI) cockpit for developers interacting with GitHub across multiple repositories, squads, and local workspaces.

It merges **remote GitHub state** (pull requests, unresolved review threads, team activity, vulnerability alerts) with **local developer reality** (cloned workspaces, branch status, ahead/behind tracking, and instant cross-repo code search).

---

## Features

### 1. Repositories Cockpit
* **Workspace Status Awareness**: Scans your local workspace folder (default `~/workspaces/*`) and classifies branches non-destructively: `[✓ synced]`, `[↓ behind]`, `[↑ ahead]`, `[* dirty]`, or `[Not Cloned]`.
* **Dynamic Multi-Group Tagging**: Press `g` to assign repositories to one or more groups (e.g., `backend`, `payments`, `core-services`). Group definitions are persisted immediately to `~/.config/grubby/config.yaml`.
* **Developer Ergonomics**:
  * `c` clones uncloned repositories into `~/workspaces/{repo}` using `gh repo clone`.
  * `e` launches the local repository directly in your `$EDITOR` (Neovim, VS Code, etc.).
  * `u` safely fast-forwards clean `behind` repositories (`git merge --ff-only`).
  * `o` opens the repository in your browser.

### 2. Pull Request Dashboard
* **Actionable Focus**: Prioritizes open pull requests across your active organization or squad.
* **Staleness Indicator**: Pull requests inactive for >3 days are flagged with a prominent warning badge `[!]`.
* **Unresolved Comments Counter**: Instant visibility into review progress (`💬 {unresolved}/{total}`).
* **Review States**: Clear color-coded badges for `APPROVED`, `CHANGES_REQ`, and `REVIEW_REQ`.
* **Copilot Review**: Press `p` to automatically request a GitHub Copilot code review.
* **Local Branch Checkout**: Press `c` to checkout the PR branch into your local workspace.

### 3. Squad Activity Feed
* **Watched Squad Focus**: Group squad members together and track what they are actively shipping across the organization (PR opens, merges, comments, commits).
* **Interactive Squad Management**:
  * `s` opens the **Squad Switcher Modal** to pivot between watched squads.
  * `a` opens the **Add Member Modal** to add GitHub handles on the fly with instant persistence to `config.yaml`.
  * `d` removes members from the active squad.

### 4. Cross-Repo Code Search & Live Syntax Preview
* **Split-Pane Search Cockpit**:
  * **Left Pane (Interactive Tree)**: Hierarchical tree navigator (`Repository` → `File` → `Line Match`) navigable via `j`/`k`, with collapsible branches (`h`/`l`/`Space`).
  * **Right Pane (Chroma Syntax Preview)**: Live code preview powered by `alecthomas/chroma/v2` with line numbers and distinct matched line pointer (`▶ 42 │`).
* **Zero-Index Overhead**: Powered by `ripgrep --json` streaming under the hood. Queries across dozens of local repositories execute in under 80ms while respecting `.gitignore`.
* **Jump to Editor**: Press `Enter` on any match to open your editor directly at that file and line number.

### 5. Notifications & macOS Desktop Integration
* **Header Notification Bubble**: Dynamic visual indicator (`🔔 {n} unread`) tracking unread GitHub notifications in real time.
* **macOS Native Notification Banners**:
  * **GitHub Ingestion**: Automatically fires native macOS notification banners with chimes whenever a refresh discovers new unread notifications.
  * **Window & Task Dispatch**: Triggers native desktop notification bubbles when opening dedicated browser tabs, launching Neovim in iTerm2, spawning shell sessions, or completing repository clones and fast-forwards.

### 6. Terminal-Responsive Dynamic Tables
* **Adaptive Column Sizing**: All views (Repositories, Pull Requests, Squad Activity, Security) compute layout widths dynamically from the terminal dimensions (`m.Width`), maximizing readability and eliminating awkward text wrapping or truncated paths on larger monitors.

### 7. Rate-Limit Friendly & Local-First
* **Batched GraphQL Queries**: Eliminates REST N+1 query bottlenecks by consolidating repositories, PRs, review threads, and CI checks into single GraphQL round-trips.
* **Automatic Credentials**: Re-uses your existing `gh auth token` session without requiring manual PAT setup.
* **Live Quota Monitoring**: Tracks your remaining API rate limit and reset window in the top status bar.

---

## Installation

### Homebrew (macOS & Linux)

The easiest way to install Grubby on macOS or Linux is via Homebrew:

```bash
# Install directly from the tap:
brew install simonbronner/tap/grubby

# Or tap first and install:
brew tap simonbronner/tap
brew install grubby
```

To upgrade in the future:
```bash
brew update
brew upgrade grubby
```

### Pre-built Binaries (GitHub Releases)

Download pre-compiled binaries from [GitHub Releases](https://github.com/simonbronner/grubby/releases):
* **macOS**: Apple Silicon (`darwin_arm64`) & Intel (`darwin_amd64`)
* **Linux**: x86_64 (`linux_amd64`) & ARM64 (`linux_arm64`)
* **Windows**: x86_64 (`windows_amd64`)

### Build from Source

#### Prerequisites
* **Go**: 1.21 or later
* **GitHub CLI (`gh`)**: Authenticated via `gh auth login` with `repo` and `read:org` scopes.
* **Ripgrep (`rg`)**: Recommended for real-time code search (`brew install ripgrep`).

```bash
git clone https://github.com/simonbronner/grubby.git
cd grubby
go build -o grubby ./cmd/grubby
```

To install directly to `$GOPATH/bin`:
```bash
go install ./cmd/grubby
```

---

## Quick Start

Launch Grubby with your default GitHub user / org:
```bash
grubby
```

Specify a target GitHub organization or custom workspace path:
```bash
grubby --org acme-corp --workspace ~/workspaces
```

---

## Keyboard Shortcuts

Press `?` at any time inside Grubby to display the contextual keybindings overlay.

### Global
| Key | Action |
| :--- | :--- |
| `1` – `5` or `Tab` | Switch between primary tabs |
| `r` | Refresh data from GitHub |
| `?` | Toggle help dialog overlay |
| `q` or `Ctrl+C` | Quit |

### Tab 1: Repositories
| Key | Action |
| :--- | :--- |
| `j` / `k` or `↑` / `↓` | Select repository |
| `g` | Open **Manage Groups Modal** (Space to toggle, `+` to add new group) |
| `c` | Clone repository to `~/workspaces/{repo}` |
| `e` | Open local repository in `$EDITOR` |
| `t` | Open repository in a new shell (iTerm2 on macOS) |
| `o` | Open repository in web browser |
| `S` | Open repository Security (Dependabot) alerts directly in browser |
| `u` | Safe fast-forward clean `behind` repository |

### Tab 2: Pull Requests
| Key | Action |
| :--- | :--- |
| `j` / `k` or `↑` / `↓` | Select pull request |
| `o` | Open pull request in browser |
| `p` | Trigger GitHub Copilot code review |
| `c` | Checkout pull request branch in local workspace |

### Tab 3: Squad Activity
| Key | Action |
| :--- | :--- |
| `s` | Open **Squad Switcher Modal** |
| `a` | Open **Add Member Modal** |
| `d` | Remove member from watched squad |
| `o` | Open activity target in browser |

### Tab 4: Code Search
| Key | Action |
| :--- | :--- |
| `/` | Focus search input box |
| `j` / `k` or `↑` / `↓` | Navigate results tree |
| `h` / `l` or `Space` | Collapse / expand repository or file branch |
| `Tab` | Switch focus between Tree Navigator and Code Preview |
| `Enter` | Open selected match in `$EDITOR` at exact line number |

### Tab 5: Notifications
| Key | Action |
| :--- | :--- |
| `j` / `k` or `↑` / `↓` | Select notification thread |
| `Enter` or `o` | Open notification in dedicated browser and mark as read |
| `m` | Mark selected notification as read |
| `M` | Mark all notifications as read |

---

## Configuration (`~/.config/grubby/config.yaml`)

Grubby automatically generates and maintains `~/.config/grubby/config.yaml`. Actions performed in the TUI (adding groups, adding squad members, switching watched squads) write back to this file atomically:

```yaml
version: 1
default_org: "acme-corp"
workspace_dir: "~/workspaces"
editor: "nvim" # or "code", fallback to $EDITOR

active_squad: "checkout-squad"

squads:
  - name: "checkout-squad"
    display_name: "Checkout Squad"
    members:
      - "alice"
      - "bob"
      - "charlie"

groups:
  - name: "payments"
    display_name: "Payments Solution"
    repos:
      - "api-payments"
      - "api-billing"

search:
  engine: "ripgrep"
  ignore_dirs:
    - ".git"
    - "node_modules"
    - "vendor"
    - "dist"
```

---

## Architecture & Technology Stack

* **TUI Framework**: Built on the [Charm](https://charm.sh) ecosystem using [Bubble Tea](https://github.com/charmbracelet/bubbletea) (The Elm Architecture in Go) and [Lipgloss](https://github.com/charmbracelet/lipgloss).
* **Syntax Highlighting**: [Chroma v2](https://github.com/alecthomas/chroma) (pure Go syntax engine with Dracula styling).
* **Search Engine**: Parallel streaming `ripgrep` runner parsing structured JSON Lines.
* **Git Engine**: Safe non-destructive status inspection and fast-forwarding (`git rev-list`, `git merge --ff-only`).

---

## Releasing & Multi-Platform Automation

Grubby includes fully automated release workflows for multi-platform builds, GitHub Releases, and Homebrew tap publishing.

### 1. Tagging a Release
Use the release helper script to automatically increment the next version tag (`v1`, `v2`, `v3`, etc.) and push it to GitHub:

```bash
# Auto-detects the next release tag (e.g. v1 -> v2) and prompts for confirmation
./scripts/release.sh

# Or trigger a specific tag directly:
./scripts/release.sh v1
```

### 2. GitHub Actions Automation
When a `v*` tag is pushed:
1. GitHub Actions triggers `.github/workflows/release.yml`.
2. Compiles cross-platform binaries for Darwin (`amd64`, `arm64`), Linux (`amd64`, `arm64`), and Windows (`amd64`).
3. Packages `.tar.gz` and `.zip` archives and computes SHA256 checksums (`checksums.txt`).
4. Generates a matching `Formula/grubby.rb` with the archive checksums.
5. Creates the GitHub Release with attached binaries, checksums, and auto-generated release notes.
6. Synchronizes the Homebrew tap repository [`simonbronner/homebrew-tap`](https://github.com/simonbronner/homebrew-tap).

### 3. Homebrew Tap Synchronization
To sync or update the tap formula locally at any time:
```bash
./scripts/update-tap.sh v1
```

---

## License

[MIT](LICENSE)

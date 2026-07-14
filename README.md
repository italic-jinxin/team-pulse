# TeamPulse

TeamPulse is a local-first engineering team insights dashboard that synchronizes GitHub repository activity, analyzes risks in the pull request workflow, and generates engineering reports that can be shared directly with management, the team, or used in standups.

Its goal is not to become another developer monitoring platform. Instead, it helps teams quickly answer a few key questions:

- What has the team been working on recently?
- Which pull requests are blocking delivery?
- Which repositories, contributors, or process signals require attention?
- How should engineering progress be communicated to stakeholders this week?

<img width="1708" height="940" alt="demo" src="https://github.com/user-attachments/assets/e21a2ec0-4e17-4f50-824f-1e1ce15fbd1e" />



## Core Capabilities

### GitHub Activity Sync

- Synchronizes commits, pull requests, reviews, and CI statuses from the last 30 days.
- Supports selecting and synchronizing multiple repositories in one operation.
- Displays the current stage, active repository, repository index, elapsed time, and failure reason during synchronization.
- Supports authentication through GitHub CLI or a temporarily entered fine-grained personal access token.

### Engineering Dashboard

- Overview: Engineering health, risks, pull requests, and team activity summary.
- Activity: Recent engineering events grouped by repository, contributor, and activity type.
- Pull Requests: Pull request status, reviews, CI, change size, and risk indicators.
- Team: Contributor activity signals over a selected time range, without reducing evaluation to a simple leaderboard.
- Repositories: Repository activity, CI health, open pull requests, and recent events.
- Risks: Aggregated signals such as stale pull requests, pending reviews, CI failures, and oversized pull requests.
- Reports: Generates Markdown weekly reports, daily reports, and risk reports.
- Settings: GitHub connection, repository synchronization, notifications, data, and local service status.

### Configurable Risk Rules

The following rules are currently configurable:

- Waiting review threshold
- Stale PR days
- Large PR line threshold
- CI failure threshold

These rules are stored locally and help teams evaluate risk according to their own delivery cadence.

### Report Templates

Reports support multiple structures:

- Executive summary
- Engineering detail
- Risk-focused
- Standup-ready

Generated reports are saved as Markdown and can be copied or downloaded.

### Local Notifications

Browser-based local notifications are supported for:

- High risk detected
- Sync failed
- Sync success
- Weekly report reminder

## Technology Stack

| Layer | Technology |
| --- | --- |
| Backend | Go 1.22, chi, SQLite |
| Frontend | React, TypeScript, Vite, Tailwind CSS |
| Data Fetching | TanStack Query |
| Desktop | SwiftUI menu bar app |
| Storage | Local SQLite database |
| Distribution | Embedded web assets + local Go server |

## Project Structure

```text
.
├── cmd/teampulse/              # Go server entry point
├── internal/app/               # API routes, GitHub sync, reports, risks, SQLite schema
├── web/                        # React + Vite frontend
│   ├── src/app/                # App shell and navigation
│   ├── src/components/         # Shared UI components
│   ├── src/lib/                # API client, formatting, preferences
│   └── src/pages/              # Dashboard pages
├── desktop/macos/              # SwiftUI menu bar launcher
├── dist/                       # Packaged macOS app artifacts, when built
├── Makefile                    # Common build and run targets
└── README.md
```

## Requirements

Required:

- Go 1.22+
- Node.js 20+
- npm

Optional:

- GitHub CLI, for `gh auth login`
- Swift 5.9+ / Xcode Command Line Tools, for building the macOS menu bar app

## Quick Start

### 1. Authenticate with GitHub

GitHub CLI is recommended:

```bash
gh auth login
```

Alternatively, enter a fine-grained personal access token on the TeamPulse Settings page. Tokens entered through the UI are stored only in the current process memory and are not written to the local database.

### 2. Start the Application

```bash
make run
```

After the service starts, it prints output similar to:

```json
{"event":"server_ready","url":"http://127.0.0.1:19421"}
```

Open the URL, go to Settings, select repositories, and start synchronization.

## Development Mode

### Backend

```bash
make server
./build/teampulse-server
```

The default address is:

```text
http://127.0.0.1:19421
```

If the port is already in use, the service automatically searches for an available port between `19421` and `19521`.

You can also specify parameters explicitly:

```bash
./build/teampulse-server -host 127.0.0.1 -port 19421 -data-dir ./tmp/data
```

### Frontend

```bash
cd web
npm install
npm run dev
```

The Vite development server proxies `/api` requests to the local Go service. POST requests must pass same-origin validation, so access the API through the Vite application rather than calling it directly from an arbitrary external origin.

### Build Production Frontend Assets

```bash
cd web
npm run build
```

Build output is written to:

```text
internal/app/webdist/
```

The Go service embeds these static assets through `embed.FS`.

## Common Commands

```bash
make web       # Install frontend dependencies and build production frontend assets
make server    # Build the frontend and compile the Go server
make run       # Build and start the local service
make test      # Run Go tests and build the frontend
make macos     # Build the server and SwiftUI macOS launcher
make clean     # Remove build output, node_modules, and Swift build caches
```

## macOS App

The project includes a SwiftUI menu bar app that:

- Starts the local TeamPulse server.
- Opens the Dashboard automatically.
- Supports restarting the server.
- Opens the local data directory.
- Exits the application and stops the server.

Build it with:

```bash
make macos
```

The repository may also contain prepackaged artifacts:

```text
dist/TeamPulse.app
dist/TeamPulse-macOS.zip
```

After rebuilding the frontend or Go backend, repackage the macOS app to ensure it includes the latest `teampulse-server` binary and web assets.

## Data Storage

By default, TeamPulse stores data locally at:

```text
~/Library/Application Support/TeamPulse/
```

Main files and directories:

| Path | Description |
| --- | --- |
| `teampulse.db` | Main SQLite database |
| `reports/` | Locally generated Markdown reports |
| `run/server.json` | Current server runtime status |
| `run/server.pid` | Current server process ID |
| `logs/` | Reserved log directory |
| `cache/` | Reserved cache directory |
| `backups/` | Reserved backup directory |

## Security and Privacy

TeamPulse is designed with the security boundary of a local developer tool:

- The HTTP service binds to `127.0.0.1` by default.
- Non-local access is rejected.
- Non-GET requests are subject to Origin validation to prevent cross-site requests.
- GitHub personal access tokens are not persisted.
- Data is written only to local SQLite storage by default.
- Reports are generated locally and are not uploaded to a cloud service.

## API Overview

### System

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/health` | Service health check |
| `GET` | `/api/app/status` | Application runtime status |
| `POST` | `/api/system/shutdown` | Request local service shutdown |

### GitHub

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/github/auth/status` | GitHub authentication status |
| `POST` | `/api/github/auth/token` | Set a temporary personal access token |
| `DELETE` | `/api/github/auth` | Clear the current in-memory token |
| `GET` | `/api/github/repositories` | List accessible GitHub repositories |

### Sync and Data

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/repositories` | List synchronized repositories |
| `POST` | `/api/repositories/sync` | Start a repository synchronization job |
| `GET` | `/api/jobs` | List synchronization jobs |
| `GET` | `/api/jobs/{id}` | Get details for a synchronization job |
| `GET` | `/api/activity` | List activity events |
| `GET` | `/api/members` | Contributor activity statistics |
| `GET` | `/api/pull-requests` | List pull requests |

### Risks

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/risks` | List risk signals |
| `PATCH` | `/api/risks/{id}` | Update risk status |
| `GET` | `/api/risk-rules` | Get risk rules |
| `PUT` | `/api/risk-rules` | Update risk rules |

### Reports

| Method | Endpoint | Description |
| --- | --- | --- |
| `POST` | `/api/reports/generate` | Generate a report |
| `GET` | `/api/reports` | List report history |
| `GET` | `/api/reports/{id}` | Get report details |

## Troubleshooting

### API Returns `403 Forbidden`

Common causes:

- The request did not originate from `127.0.0.1` or `localhost`.
- The `Origin` for a POST, PUT, PATCH, or DELETE request does not match the backend service URL.
- The API was called directly without using the Vite proxy.

Resolution:

- During development, access the application through the Vite page.
- Confirm the actual backend URL, for example `http://127.0.0.1:19421`.
- If the service falls back to another port, use the URL printed by the server.

### GitHub Repository List Is Empty

Possible causes:

- GitHub CLI is not authenticated.
- The personal access token does not have sufficient permissions.
- The fine-grained token is not authorized for the target organization or repository.

Resolution:

```bash
gh auth status
gh auth login
```

Alternatively, enter a personal access token with repository read access in Settings.

### Synchronization Takes a Long Time

Synchronization duration depends on:

- Number of repositories
- Number of pull requests from the last 30 days
- GitHub API response time
- GitHub rate limits

The synchronization panel displays the current stage, active repository, repository progress, and failure reason. If a job fails, first check token permissions, repository access, and GitHub rate limits.

## Current Limitations

- The current focus is local, single-user usage.
- GitHub OAuth device flow is not yet implemented.
- Persistent token storage in macOS Keychain is not yet implemented.
- Automatic scheduled background synchronization remains a future enhancement.
- Packaged applications must be signed and notarized before distribution to other users.

## Roadmap

- GitHub OAuth device flow
- Keychain token storage
- Automatic scheduled synchronization
- More granular GitHub rate limit visibility
- Real-time synchronization events through SSE or WebSocket
- Configurable report export formats
- Signed and notarized macOS distribution

## License

This repository does not currently declare an open-source license. Add an explicit license before publishing or distributing it.

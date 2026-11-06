# Canvas LMS Grade & Assignment Scraper

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Platform Support](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat)]()
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A high-performance, standalone Go tool designed to extract courses, grades, category weights, assignments, and submission details from any **Canvas LMS** institution (`https://<school>.instructure.com`). Supports direct student accounts as well as **Parent / Observer accounts** observing multiple students.

Exports to **JSON**, **CSV**, **interactive single-file HTML reports** (with Dark Mode & responsive tables), **AI-optimized Markdown summaries**, and formatted color terminal tables.

---

## 🌟 Key Features

- 🎓 **Full Gradebook Extraction**: Captures overall course scores, letter grades, grading periods, assignment category weights (e.g. *Tests 55%*, *Quizzes 35%*, *Homework 10%*), and individual assignment statuses.
- 👨‍👩‍👧‍👦 **Parent / Observer Support**: Automatically discovers all observed students (e.g., observing multiple children) and maps their enrollments and submissions into clean, per-student breakdowns.
- ⚡ **Smart Two-Tier Session & Data Caching**:
  - **Session Persistence (`storage_state.json`)**: Performs browser login once via headless Chrome (`chromedp`), extracts authentication cookies in Playwright-compatible format, and reuses them directly over lightweight HTTP without launching Chrome again.
  - **Results Caching (`CACHE_TIMER`)**: Caches scraped Canvas data locally with precise age and countdown tracking, allowing instant filtering, sorting, and report regeneration without touching Canvas servers.
- 🛡️ **Polite Scraping & Anti-Overload**: Combines assignment group and assignment queries into single API requests, adds jittered pacing delays between course requests, and supports skipping non-graded homerooms/advisories (`SKIP_NONGRADED=true`).
- 🌐 **Interactive Single-File HTML Reports (`--web`)**: Generates a self-contained, dependency-free HTML dashboard with automatic Light/Dark mode, sticky headers, responsive stacked-card mobile layout, and automatically opens it in your default browser.
- 🤖 **AI-Ready Markdown Export (`--markdown`)**: Produces structured Markdown reports (`canvas_grades.md`) with executive summaries, pre-extracted action items (missing, overdue, or late work), and tabular breakdowns ready for LLM consumption.
- 🎯 **Deep Filtering & Due Date Windows**: Filter by student, course, future due dates, lookahead windows (`--due-within 7d`), lookback windows (`--due-past 14d`), missing/overdue thresholds (`--missing-days 2`), score thresholds (`--below 75`), and minimum category weights.
- 🔄 **Comprehensive Sorting**: Sort assignments and courses by due date (`date`, `date-desc`), urgency status (`status`), score percentage (`score`, `score-desc`), letter grade (`grade`), or category weight (`weight`).
- 📂 **Multi-Location Config Discovery**: Searches `.env` configuration across your current working directory, the binary's directory, and `~/.config/canvas-scraper/.env`, with full shell environment variable override support.

---

## 🏗️ Architecture & How It Works

```
┌────────────────────────────────────────────────────────────────────────┐
│                          Canvas Scraper (Go)                           │
└────────────────────────────────────────────────────────────────────────┘
          │                                              │
          ▼ (If session expired / fresh)                 ▼ (If session valid)
┌───────────────────────────┐                  ┌───────────────────────────┐
│ Headless Chrome (Chromedp)│                  │ Go HTTP Client (REST API) │
│  - Authenticates securely │                  │  - Fetches users/observees│
│  - Saves session cookies  │───────►──────────│  - Fetches courses/scores │
│    to storage_state.json  │                  │  - Fetches groups/submits │
└───────────────────────────┘                  └───────────────────────────┘
                                                             │
                                                             ▼
                                               ┌───────────────────────────┐
                                               │ Local Cache Envelope      │
                                               │  (.cache_raw.json)        │
                                               └───────────────────────────┘
                                                             │
                              ┌──────────────────────────────┼──────────────────────────────┐
                              ▼                              ▼                              ▼
                 ┌───────────────────────────┐ ┌───────────────────────────┐ ┌───────────────────────────┐
                 │       Terminal UI         │ │     Data Files (CSV/JSON) │ │   Web / AI Deliverables   │
                 │  - Color Overview Table   │ │  - canvas_grades.json     │ │  - canvas_grades.html (--web)│
                 │  - Course-by-course breakdown- courses_summary.csv    │ │  - canvas_grades.md (--markdown)
                 │  - Live countdown banner  │ │  - assignment_weights.csv │ └───────────────────────────┘
                 └───────────────────────────┘ │  - assignments_detail.csv │
                                               └───────────────────────────┘
```

---

## 🚀 Quick Start

### 1. Download or Build

Download a precompiled binary for your operating system or compile from source:

```bash
# macOS (Apple Silicon / arm64)
chmod +x canvas-scraper

# Linux (x86_64)
chmod +x canvas-scraper-linux

# Windows
# Run canvas-scraper.exe from Command Prompt or PowerShell
```

### 2. Configure Credentials

Create a `.env` file either in the folder you run the command from, or globally in `~/.config/canvas-scraper/.env`:

```bash
cp .env.example .env
```

Edit `.env`:

```ini
# Base URL of your Canvas LMS instance (Required)
CANVAS_URL=https://mtparanschool.instructure.com

# Login credentials
CANVAS_USER_ID=your_username_or_email
CANVAS_PASSWORD=your_password

# Where exported files are saved (defaults to "output", supports ~/... paths)
OUTPUT_DIR=output

# Local cache duration before hitting Canvas again (e.g. 10m, 1h, 24h, or 0 to disable)
CACHE_TIMER=10m

# Skip non-graded / advisory / homeroom courses
SKIP_NONGRADED=true

# Exclude "Not Graded" and 0% weight categories that do not impact final grades
SKIP_UNGRADED_ITEMS=true
```

> **Security Note**: Never commit your `.env` or `storage_state.json` file to GitHub or version control. Both are included in `.gitignore`.

### 3. Run the Scraper

```bash
./canvas-scraper
```

On the first run, the scraper launches a headless Chrome instance to log in, saves the authenticated session to `storage_state.json`, queries the Canvas REST API, and displays the gradebook summary.

Subsequent runs within the `CACHE_TIMER` window return instantly from the local cache without making any network calls!

---

## ⚙️ Configuration & Priority Order

Configuration settings can be provided in multiple places. The scraper merges them in the following order (highest priority first):

1. **Environment Variables**: Variables passed directly in the shell (e.g. `export CANVAS_URL=...` or `CACHE_TIMER=5m ./canvas-scraper`).
2. **`./.env`**: Local `.env` file in the current working directory.
3. **`<executable-dir>/.env`**: `.env` file located alongside the compiled `canvas-scraper` binary.
4. **`~/.config/canvas-scraper/.env`**: User-wide configuration file (`C:\Users\<user>\.config\canvas-scraper\.env` on Windows).

### Available Environment / `.env` Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| `CANVAS_URL` | String | *Required* | Base URL of your Canvas instance (e.g. `https://yourschool.instructure.com`) |
| `CANVAS_USER_ID` | String | *Required** | Your Canvas login email or username (*not required if `CANVAS_API_TOKEN` is used) |
| `CANVAS_PASSWORD` | String | *Required** | Your Canvas login password |
| `CANVAS_API_TOKEN` | String | Optional | Canvas personal access token (bypasses browser login entirely) |
| `OUTPUT_DIR` | String | `output` | Directory where JSON, CSV, HTML, MD, and cache files are stored (supports `~/...`) |
| `CACHE_TIMER` | Duration | `10m` | Local data cache duration (`30s`, `10m`, `2h`, `7d`, or `0` to disable) |
| `SKIP_NONGRADED` | Boolean | `true` | Skip non-graded courses (counseling, homeroom, study hall) |
| `SKIP_UNGRADED_ITEMS` | Boolean | `true` | Filter out "Not Graded" / 0% weight categories and assignments |
| `SORT_BY` | String | Empty | Default sorting order across reports (`date`, `status`, `score`, `grade`, `weight`) |
| `HIDE_FUTURE_DUE` | Boolean | `false` | Exclude ungraded assignments with future due dates |
| `FUTURE_ONLY` | Boolean | `false` | Show only assignments with future due dates |
| `DUE_WITHIN` | Duration | Empty | Show only assignments due within duration (e.g. `7d`, `14d`, `48h`) |
| `DUE_PAST` | Duration | Empty | Show only assignments due in the past duration (e.g. `14d`, `30d`) |
| `HIDE_NO_DUE_DATE` | Boolean | `false` | Exclude ungraded assignments that have no due date |
| `MISSING_AFTER_DAYS`| Integer | `-1` | Show items missing or unsubmitted $\ge N$ days past due (`0` = any overdue) |
| `STATUS_FILTER` | String | Empty | Comma-separated status filters (`missing,late,graded,ungraded,excused`) |
| `SCORE_BELOW` | Float | `0` | Show only graded assignments with percentage score below $X\%$ (e.g. `75`) |
| `MIN_CATEGORY_WEIGHT` | Float | `0` | Show only assignments in categories weighted $\ge X\%$ (e.g. `30`) |

---

## 💻 CLI Flags & Usage Examples

Run `./canvas-scraper --help` at any time to view the built-in reference.

### General & Output Flags

```bash
# Bypass local cache and force live Canvas fetch
./canvas-scraper --fresh

# Launch visible browser window (for MFA / Captcha debugging)
./canvas-scraper --headed

# Generate an interactive HTML report and open it in your browser
./canvas-scraper --web

# Generate an AI-ready Markdown report (output/canvas_grades.md)
./canvas-scraper --markdown

# Custom output destination
./canvas-scraper --output-dir ~/Desktop/grades
```

### Filtering Flags

All filtering flags operate instantaneously against your cached data without re-querying Canvas:

```bash
# Filter by student name (Observer accounts)
./canvas-scraper --student Emma
./canvas-scraper --student Tyler

# Filter by course name or Course ID
./canvas-scraper --course "Algebra"
./canvas-scraper --course 1114

# Upcoming work only (lookahead window)
./canvas-scraper --due-within 7d
./canvas-scraper --future-only

# Problem work: Missing or unsubmitted work overdue by 2+ days
./canvas-scraper --missing-days 2

# Low grades investigation: Show graded items below 75%
./canvas-scraper --below 75

# Focus on major assessments (Summatives / Tests >= 30% weight)
./canvas-scraper --min-weight 30

# Filter by submission status
./canvas-scraper --status missing,late
```

### Sorting Flags (`--sort <field>`)

```bash
# Sort assignments by due date (newest/upcoming first)
./canvas-scraper --sort date-desc

# Sort assignments by urgency (Missing/Overdue -> Late -> Submitted -> Graded)
./canvas-scraper --sort status

# Sort courses & assignments from lowest score to highest (spot trouble areas first)
./canvas-scraper --sort score

# Sort courses & assignments by letter grade (A -> F)
./canvas-scraper --sort grade
```

---

## 📂 Exported Output Formats

All outputs are written to the configured `OUTPUT_DIR` (default: `output/`):

| File | Format | Purpose |
|---|---|---|
| `canvas_grades.html` | HTML5 | **Interactive Web Dashboard** generated by `--web`. Self-contained with zero external CSS/JS dependencies, responsive container queries, sticky column headers, and dark mode support. |
| `canvas_grades.md` | Markdown | **AI / LLM Analysis Document** generated by `--markdown`. Contains report metadata, an executive summary table, a pre-extracted **Action Items** section (all missing, overdue, and late items), and full per-course breakdowns. |
| `canvas_grades.json` | JSON | Complete structured export containing raw student IDs, course metadata, assignment group weighting rules, and individual assignment submission histories. |
| `courses_summary.csv` | CSV | High-level spreadsheet of all courses, students, grading periods, current percentage scores, and letter grades. |
| `assignment_weights.csv` | CSV | Breakdown of every assignment group/category and its exact percentage weight per course. |
| `assignments_detail.csv` | CSV | Comprehensive table of every single assignment, due date, points earned, points possible, percentage, category, status, and grading date. |
| `.cache_raw.json` | JSON | Internal cache envelope recording raw scrape data and precise ISO timestamps for `CACHE_TIMER` validation. |

---

## 🛠️ Cross-Compilation & Building from Source

### Prerequisites
- [Go 1.22+](https://go.dev/dl/) installed.
- Google Chrome, Chromium, or Microsoft Edge installed on the machine running the scraper (required for initial headless authentication).

### Build Binaries

You can cross-compile single-file native executables for macOS, Linux, and Windows from any development machine:

```bash
# Clone the repository
git clone https://github.com/<your-username>/canvas-scraper.git
cd canvas-scraper

# Download Go dependencies
go mod tidy

# macOS (Apple Silicon arm64)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o canvas-scraper main.go

# macOS (Intel amd64)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o canvas-scraper-intel main.go

# Windows (64-bit .exe)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o canvas-scraper.exe main.go

# Linux (64-bit amd64)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o canvas-scraper-linux main.go
```

---

## ⏰ Automated Scheduling & Cron

Because `canvas-scraper` is a single static executable, you can easily run it on a recurring schedule to generate fresh daily grade summaries or alerts.

### Example: macOS / Linux Cron Job
Run every weekday at 4:00 PM and output an updated HTML and Markdown report:

```bash
# Edit crontab
crontab -e

# Add entry (replace with your absolute paths)
0 16 * * 1-5 cd /Users/username/canvas-scraper && ./canvas-scraper --markdown >> scraper.log 2>&1
```

---

## ❓ Frequently Asked Questions (FAQ)

#### Q: Does this work with Canvas accounts using Multi-Factor Authentication (MFA / 2FA)?
**Yes.** If your school requires 2FA or Captcha on login, run the scraper once with the `--headed` flag:
```bash
./canvas-scraper --headed
```
A visible browser window will appear. Complete your authentication or approval prompt. Once login succeeds, `storage_state.json` saves your session cookies, and all future runs can proceed fully headless.

#### Q: Can I use an API Token instead of user credentials?
**Yes.** If your school's Canvas policy allows generating Personal Access Tokens (under *Account* $\rightarrow$ *Settings* $\rightarrow$ *+ New Access Token*), set `CANVAS_API_TOKEN=your_token_here` in `.env`. The scraper will skip the browser login entirely and query the Canvas REST API directly.

#### Q: How does it calculate overall course scores?
Canvas stores several enrollment scores. The scraper checks the **current grading period score** first (e.g. Quarter 1, Semester 1), then falls back to `computed_current_score` or `current_score`.

#### Q: What does "Asgs" stand for in the summary tables?
**"Asgs" stands for Assignments.** It shows the count of active assignments displayed for that course based on your active filters.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE). You are free to use, modify, and distribute it in personal or commercial environments.

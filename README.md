# Canvas LMS Grade & Assignment Scraper (Go)
### Mount Paran Christian School (`https://mtparanschool.instructure.com`)

A standalone Go scraper that extracts all class grades, assignments, assignment category weights, and submission details directly from Canvas LMS—supporting both Student accounts and Parent/Observer accounts.

---

## 🌟 What This Scraper Extracts

1. **Overall Course Grades**:
   - Student name (automatically detects observed students for Parent/Observer accounts)
   - Course name, course code, and grading period
   - Current percentage score (e.g. `85.01%`) and letter grade (e.g. `B`)
2. **Assignment Groups & Weights**:
   - Exact category weights (e.g. `Tests: 55%`, `Quizzes: 35%`, `Homework: 10%`)
3. **Assignments & Submissions**:
   - Assignment title, category group, and category weight %
   - Due date & graded timestamp
   - Points possible, earned score, and calculated percentage
   - Submission status (`Graded`, `Missing`, `Late`, `Excused`, `Not Submitted`)

---

## 🚀 Quick Start

### 1. Set Up Your Credentials

Create a `.env` file in the same folder as the binary:

```bash
cp .env.example .env
```

Edit `.env` with your Canvas User ID (or email) and password:

```ini
CANVAS_URL=https://mtparanschool.instructure.com
CANVAS_USER_ID=your_email_or_user_id
CANVAS_PASSWORD=your_password
```

---

### 2. Run the Scraper

#### Run the compiled binary directly:
```bash
# macOS
./canvas-scraper

# Linux
./canvas-scraper-linux

# Windows (Command Prompt / PowerShell)
.\canvas-scraper.exe
```

#### Or run from source with Go:
```bash
go run main.go
```

> **Session Caching**: After the first login, the scraper saves session cookies to `storage_state.json`. Subsequent runs reuse the session cookies directly via Go HTTP requests without even launching a browser!

---

## 🛠️ Cross-Compiling for macOS, Windows, and Linux

From your Mac, you can compile standalone single-file binaries for any OS:

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o canvas-scraper main.go

# Windows (64-bit .exe)
GOOS=windows GOARCH=amd64 go build -o canvas-scraper.exe main.go

# Linux (64-bit)
GOOS=linux GOARCH=amd64 go build -o canvas-scraper-linux main.go
```

---

## 📂 Exported Files

Every run exports the following files into `output/`:

| File | Description |
|---|---|
| `output/courses_summary.csv` | Summary of all courses grouped by student, current score %, and letter grade |
| `output/assignment_weights.csv` | Course-by-course category breakdown and percentage weights |
| `output/assignments_detail.csv` | Every assignment, due date, score, possible points, % score, and status |
| `output/canvas_grades.json` | Full structured JSON representation |

---

## ⚙️ Command-Line Flags

| Flag | Description |
|---|---|
| `--headed` | Launches browser with a visible window (useful for 2FA/captcha or debugging) |
| `--fresh` | Ignores cached `storage_state.json` and forces a fresh browser login |
| `--student <name>` | Filters results by student name (e.g. `--student Emma` or `--student Tyler`) |
| `--course <name_or_id>` | Filters results for a specific course (e.g. `--course "Algebra I"`) |
| `--output-dir <path>` | Custom directory to save output files (default: `output`) |

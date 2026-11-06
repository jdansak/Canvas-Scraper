package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	SessionFile       = "storage_state.json"
	DefaultOutputDir  = "output"
	ColorReset        = "\033[0m"
	ColorBold         = "\033[1m"
	ColorDim          = "\033[2m"
	ColorRed          = "\033[31m"
	ColorGreen        = "\033[32m"
	ColorYellow       = "\033[33m"
	ColorBlue         = "\033[34m"
	ColorMagenta      = "\033[35m"
	ColorCyan         = "\033[36m"
	ColorWhite        = "\033[37m"
	ColorBoldGreen    = "\033[1;32m"
	ColorBoldYellow   = "\033[1;33m"
	ColorBoldRed      = "\033[1;31m"
	ColorBoldCyan     = "\033[1;36m"
	ColorBoldMagenta  = "\033[1;35m"
	ColorBoldBlue     = "\033[1;34m"
	ColorBoldWhite    = "\033[1;37m"
)

// StorageState matches Playwright's storage_state.json format so sessions are interchangeable.
type StorageState struct {
	Cookies []StoredCookie `json:"cookies"`
	Origins []any          `json:"origins,omitempty"`
}

type StoredCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite,omitempty"`
}

// Output Data Structures matching Python's canvas_grades.json
type CourseGradeRecord struct {
	StudentID        int                     `json:"student_id"`
	StudentName      string                  `json:"student_name"`
	CourseID         int                     `json:"course_id"`
	CourseName       string                  `json:"course_name"`
	CourseCode       string                  `json:"course_code"`
	GradingPeriod    *string                 `json:"grading_period"`
	OverallScore     *float64                `json:"overall_score"`
	OverallGrade     *string                 `json:"overall_grade"`
	FinalScore       *float64                `json:"final_score"`
	AssignmentGroups []AssignmentGroupRecord `json:"assignment_groups"`
	Assignments      []AssignmentRecord      `json:"assignments"`
}

type AssignmentGroupRecord struct {
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	WeightPercent float64        `json:"weight_percent"`
	Rules         map[string]any `json:"rules"`
}

type AssignmentRecord struct {
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	GroupID            int      `json:"group_id"`
	GroupName          string   `json:"group_name"`
	GroupWeightPercent float64  `json:"group_weight_percent"`
	DueAt              *string  `json:"due_at"`
	PointsPossible     *float64 `json:"points_possible"`
	Score              *float64 `json:"score"`
	Grade              *string  `json:"grade"`
	Percentage         *float64 `json:"percentage"`
	Status             string   `json:"status"`
	SubmittedAt        *string  `json:"submitted_at"`
	GradedAt           *string  `json:"graded_at"`
	Late               bool     `json:"late"`
	Missing            bool     `json:"missing"`
	Excused            bool     `json:"excused"`
	OmitFromFinalGrade bool     `json:"omit_from_final_grade,omitempty"`
	GradingType        string   `json:"grading_type,omitempty"`
	HTMLURL            *string  `json:"html_url"`
}

// Canvas API Raw Models
type CanvasUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CanvasCourse struct {
	ID                     int                `json:"id"`
	Name                   string             `json:"name"`
	CourseCode             string             `json:"course_code"`
	AccessRestrictedByDate bool               `json:"access_restricted_by_date"`
	Enrollments            []CanvasEnrollment `json:"enrollments"`
}

type CanvasEnrollment struct {
	Type                              string   `json:"type"`
	UserID                            int      `json:"user_id"`
	AssociatedUserID                  int      `json:"associated_user_id"`
	CurrentGradingPeriodTitle         *string  `json:"current_grading_period_title"`
	ComputedCurrentScore              *float64 `json:"computed_current_score"`
	CurrentScore                      *float64 `json:"current_score"`
	CurrentPeriodComputedCurrentScore *float64 `json:"current_period_computed_current_score"`
	ComputedCurrentGrade              *string  `json:"computed_current_grade"`
	ComputedCurrentLetterGrade        *string  `json:"computed_current_letter_grade"`
	CurrentGrade                      *string  `json:"current_grade"`
	CurrentPeriodComputedCurrentGrade *string  `json:"current_period_computed_current_grade"`
	ComputedFinalScore                *float64 `json:"computed_final_score"`
	CurrentPeriodComputedFinalScore   *float64 `json:"current_period_computed_final_score"`
}

type CanvasAssignmentGroup struct {
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	GroupWeight float64            `json:"group_weight"`
	Rules       map[string]any     `json:"rules"`
	Assignments []CanvasAssignment `json:"assignments"`
}

type CanvasAssignment struct {
	ID                 int               `json:"id"`
	Name               string            `json:"name"`
	AssignmentGroupID  int               `json:"assignment_group_id"`
	DueAt              *string           `json:"due_at"`
	PointsPossible     *float64          `json:"points_possible"`
	OmitFromFinalGrade bool              `json:"omit_from_final_grade"`
	GradingType        string            `json:"grading_type"`
	HTMLURL            *string           `json:"html_url"`
	Submission         *CanvasSubmission `json:"submission"`
}

type CanvasSubmission struct {
	AssignmentID  int      `json:"assignment_id"`
	Score         *float64 `json:"score"`
	EnteredScore  *float64 `json:"entered_score"`
	Grade         *string  `json:"grade"`
	EnteredGrade  *string  `json:"entered_grade"`
	WorkflowState string   `json:"workflow_state"`
	SubmittedAt   *string  `json:"submitted_at"`
	GradedAt      *string  `json:"graded_at"`
	Late          bool     `json:"late"`
	Missing       bool     `json:"missing"`
	Excused       bool     `json:"excused"`
}

// CacheEnvelope stores the exact scrape timestamp alongside raw course records.
type CacheEnvelope struct {
	ScrapedAt time.Time           `json:"scraped_at"`
	Courses   []CourseGradeRecord `json:"courses"`
}

// AppConfig holds credentials, caching settings, and all course/assignment filters.
type AppConfig struct {
	CanvasURL         string
	UserID            string
	Password          string
	APIToken          string
	SkipNonGraded     bool
	SkipUngradedItems bool
	CacheTimer        time.Duration
	OutputDir         string

	// Course & Student Filters
	StudentFilter string
	CourseFilter  string

	// Assignment Filters
	HideFutureDue     bool
	FutureOnly        bool
	DueWithin         time.Duration
	DuePast           time.Duration
	HideNoDueDate     bool
	MissingAfterDays  int // -1 means disabled; >= 0 means missing or overdue by >= N days
	StatusFilter      string
	ScoreBelow        float64 // <= 0 means disabled
	MinCategoryWeight float64 // <= 0 means disabled

	// Sorting
	SortBy string
}

// dotEnvSearchPaths returns candidate .env locations in priority order (highest first):
//  1. .env in the current working directory
//  2. .env in the same directory as the executable
//  3. ~/.config/canvas-scraper/.env
func dotEnvSearchPaths() []string {
	var paths []string
	seen := make(map[string]bool)
	add := func(p string) {
		if p == "" {
			return
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		// Resolve symlinks on the directory so the same folder isn't listed twice
		// (e.g. macOS /var -> /private/var).
		if dir, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
			p = filepath.Join(dir, filepath.Base(p))
		}
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, ".env"))
	}
	if exePath, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = resolved
		}
		add(filepath.Join(filepath.Dir(exePath), ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".config", "canvas-scraper", ".env"))
	}
	return paths
}

// loadDotEnvFiles loads every .env file found in dotEnvSearchPaths.
// Precedence (highest wins):
//  1. Real environment variables already set in the shell (e.g. export CANVAS_URL=...)
//  2. .env in the current directory
//  3. .env next to the executable
//  4. ~/.config/canvas-scraper/.env
func loadDotEnvFiles() []string {
	protected := make(map[string]bool)
	for _, kv := range os.Environ() {
		if idx := strings.Index(kv, "="); idx > 0 && kv[idx+1:] != "" {
			protected[kv[:idx]] = true
		}
	}

	var loaded []string
	for _, p := range dotEnvSearchPaths() {
		if loadDotEnv(p, protected) {
			loaded = append(loaded, p)
		}
	}
	return loaded
}

// loadDotEnv reads a single .env file and sets any keys not already in `protected`.
// Keys it sets are added to `protected` so lower-priority files cannot override them.
// Returns true if the file was found and read.
func loadDotEnv(path string, protected map[string]bool) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	seenInFile := make(map[string]bool)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip surrounding quotes or inline comments
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		} else if idx := strings.Index(val, " #"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}
		if !protected[key] || seenInFile[key] {
			_ = os.Setenv(key, val)
			protected[key] = true
			seenInFile[key] = true
		}
	}
	return true
}

func parseFlexibleDuration(raw string) (time.Duration, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimRight(s, ".")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" || s == "0" || s == "false" || s == "none" {
		return 0, nil
	}
	s = strings.ReplaceAll(s, "weeks", "w")
	s = strings.ReplaceAll(s, "week", "w")
	s = strings.ReplaceAll(s, "wks", "w")
	s = strings.ReplaceAll(s, "wk", "w")
	s = strings.ReplaceAll(s, "days", "d")
	s = strings.ReplaceAll(s, "day", "d")
	s = strings.ReplaceAll(s, "hours", "h")
	s = strings.ReplaceAll(s, "hour", "h")
	s = strings.ReplaceAll(s, "hrs", "h")
	s = strings.ReplaceAll(s, "hr", "h")
	s = strings.ReplaceAll(s, "minutes", "m")
	s = strings.ReplaceAll(s, "minute", "m")
	s = strings.ReplaceAll(s, "mins", "m")
	s = strings.ReplaceAll(s, "min", "m")
	s = strings.ReplaceAll(s, "seconds", "s")
	s = strings.ReplaceAll(s, "second", "s")
	s = strings.ReplaceAll(s, "secs", "s")
	s = strings.ReplaceAll(s, "sec", "s")

	if strings.HasSuffix(s, "w") {
		if num, err := strconv.ParseFloat(strings.TrimSuffix(s, "w"), 64); err == nil && num > 0 {
			return time.Duration(num * float64(7*24*time.Hour)), nil
		}
	}
	if strings.HasSuffix(s, "d") {
		if num, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64); err == nil && num > 0 {
			return time.Duration(num * float64(24*time.Hour)), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if secs, err := strconv.Atoi(s); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid duration: %q", raw)
}

func parseEnvBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "true" || v == "1" || v == "yes"
}

func expandHomePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func loadConfigFromEnv() AppConfig {
	for _, p := range loadDotEnvFiles() {
		fmt.Printf("%sLoaded config: %s%s\n", ColorDim, p, ColorReset)
	}
	outDir := expandHomePath(os.Getenv("OUTPUT_DIR"))
	if outDir == "" {
		outDir = DefaultOutputDir
	}
	cfg := AppConfig{
		CanvasURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("CANVAS_URL")), "/"),
		UserID:            strings.TrimSpace(os.Getenv("CANVAS_USER_ID")),
		Password:          strings.TrimSpace(os.Getenv("CANVAS_PASSWORD")),
		APIToken:          strings.TrimSpace(os.Getenv("CANVAS_API_TOKEN")),
		SkipNonGraded:     parseEnvBool("SKIP_NONGRADED"),
		SkipUngradedItems: parseEnvBool("SKIP_UNGRADED_ITEMS"),
		OutputDir:         outDir,
		HideFutureDue:     parseEnvBool("HIDE_FUTURE_DUE"),
		FutureOnly:        parseEnvBool("FUTURE_ONLY"),
		HideNoDueDate:     parseEnvBool("HIDE_NO_DUE_DATE"),
		MissingAfterDays:  -1,
		StatusFilter:      strings.TrimSpace(os.Getenv("STATUS_FILTER")),
		SortBy:            strings.TrimSpace(os.Getenv("SORT_BY")),
	}

	if timerRaw := strings.TrimSpace(os.Getenv("CACHE_TIMER")); timerRaw != "" && timerRaw != "0" && !strings.EqualFold(timerRaw, "false") {
		if d, err := parseFlexibleDuration(timerRaw); err == nil {
			cfg.CacheTimer = d
		} else {
			fmt.Printf("%sWarning: Could not parse CACHE_TIMER=%q in .env (expected e.g. 30s, 10m, 1h).%s\n", ColorBoldRed, timerRaw, ColorReset)
		}
	}

	if dwRaw := strings.TrimSpace(os.Getenv("DUE_WITHIN")); dwRaw != "" {
		if strings.EqualFold(dwRaw, "future") || strings.EqualFold(dwRaw, "all") {
			cfg.FutureOnly = true
		} else if d, err := parseFlexibleDuration(dwRaw); err == nil {
			cfg.DueWithin = d
		} else if days, err := strconv.Atoi(dwRaw); err == nil && days > 0 {
			cfg.DueWithin = time.Duration(days) * 24 * time.Hour
		}
	}

	if dpRaw := strings.TrimSpace(os.Getenv("DUE_PAST")); dpRaw != "" {
		if d, err := parseFlexibleDuration(dpRaw); err == nil {
			cfg.DuePast = d
		} else if days, err := strconv.Atoi(dpRaw); err == nil && days > 0 {
			cfg.DuePast = time.Duration(days) * 24 * time.Hour
		}
	}

	if mdRaw := strings.TrimSpace(os.Getenv("MISSING_AFTER_DAYS")); mdRaw != "" && !strings.EqualFold(mdRaw, "false") && !strings.EqualFold(mdRaw, "none") {
		if days, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(mdRaw), "d")); err == nil && days >= 0 {
			cfg.MissingAfterDays = days
		}
	}

	if sbRaw := strings.TrimSpace(os.Getenv("SCORE_BELOW")); sbRaw != "" {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(sbRaw, "%"), 64); err == nil && v > 0 {
			cfg.ScoreBelow = v
		}
	}

	if mwRaw := strings.TrimSpace(os.Getenv("MIN_CATEGORY_WEIGHT")); mwRaw != "" {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(mwRaw, "%"), 64); err == nil && v > 0 {
			cfg.MinCategoryWeight = v
		}
	}

	return cfg
}

// CanvasClient handles API requests using either Session Cookies or an API Bearer Token.
type CanvasClient struct {
	BaseURL    string
	HTTPClient *http.Client
	APIToken   string
}

func NewCanvasClientWithToken(baseURL, token string) *CanvasClient {
	return &CanvasClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		APIToken: token,
	}
}

func NewCanvasClientWithCookies(baseURL string, cookies []StoredCookie) (*CanvasClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	var httpCookies []*http.Cookie
	for _, c := range cookies {
		httpCookies = append(httpCookies, &http.Cookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
		})
	}
	jar.SetCookies(u, httpCookies)

	return &CanvasClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *CanvasClient) getJSON(endpoint string, target any) error {
	fullURL := c.BaseURL + endpoint
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// Strip Canvas while(1); prefix if present
	bodyStr := strings.TrimPrefix(string(bodyBytes), "while(1);")
	return json.Unmarshal([]byte(bodyStr), target)
}

func loadSessionCookies(path string) ([]StoredCookie, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state StorageState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return state.Cookies, nil
}

func saveSessionCookies(path string, cookies []StoredCookie) error {
	state := StorageState{
		Cookies: cookies,
		Origins: []any{},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func findBrowserBinary() string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		progFiles := os.Getenv("ProgramFiles")
		progFiles86 := os.Getenv("ProgramFiles(x86)")
		candidates = []string{
			filepath.Join(progFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(progFiles86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(progFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(progFiles86, "Microsoft", "Edge", "Application", "msedge.exe"),
		}
	default:
		candidates = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge",
		}
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func authenticateWithBrowser(baseURL, userID, password string, headless bool) ([]StoredCookie, error) {
	if userID == "" || password == "" {
		return nil, fmt.Errorf("CANVAS_USER_ID and CANVAS_PASSWORD are required. Please provide them in your .env file")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	)

	if bin := findBrowserBinary(); bin != "" {
		opts = append(opts, chromedp.ExecPath(bin))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	loginURL := baseURL + "/login/canvas"
	fmt.Printf("%sFilling credentials on %s...%s\n", ColorCyan, loginURL, ColorReset)

	err := chromedp.Do(ctx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible(chromedp.CSS(`input#username, input[name="pseudonym_session[unique_id]"]`)),
		chromedp.SendKeys(chromedp.CSS(`input#username`), userID),
		chromedp.SendKeys(chromedp.CSS(`input#password`), password+"\r"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fill login form: %w", err)
	}

	fmt.Printf("%sSubmitting credentials...%s\n", ColorCyan, ColorReset)
	// Also trigger form submission via JavaScript click on submit button if Enter didn't submit
	_, _ = chromedp.Run(ctx,
		chromedp.Evaluate[bool](`(() => {
			const btn = document.querySelector('button[type="submit"], input[type="submit"]');
			if (btn) { btn.click(); return true; }
			const form = document.querySelector('form');
			if (form) { form.submit(); return true; }
			return false;
		})()`),
	)

	fmt.Printf("%sWaiting for dashboard redirect...%s\n", ColorCyan, ColorReset)
	if !headless {
		fmt.Printf("%s(Headed mode active: If Canvas prompts for 2FA, verification code, or captcha, you can complete it in the browser window)%s\n", ColorDim, ColorReset)
	}

	maxWait := 25 * time.Second
	if !headless {
		maxWait = 120 * time.Second
	}
	deadline := time.Now().Add(maxWait)
	loginSucceeded := false

	for time.Now().Before(deadline) {
		currentURL, _ := chromedp.Run(ctx, chromedp.Location())
		bodyText, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`document.body ? document.body.innerText : ""`))

		if currentURL != "" && !strings.Contains(currentURL, "/login") {
			loginSucceeded = true
			break
		}

		lowerBody := strings.ToLower(bodyText)
		if strings.Contains(lowerBody, "verify your email or password") || strings.Contains(lowerBody, "invalid username or password") {
			return nil, fmt.Errorf("Login failed: Canvas reported 'Please verify your email or password and try again.' Please verify CANVAS_USER_ID and CANVAS_PASSWORD in your .env file")
		}

		time.Sleep(1 * time.Second)
	}

	if !loginSucceeded {
		return nil, fmt.Errorf("login did not complete within %v. Try running with --headed to inspect or complete 2FA manually", maxWait)
	}

	// Extract cookies via CDP
	var storedCookies []StoredCookie
	err = chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		res, err := cdp.Call(ctx, t, network.GetCookies, network.GetCookiesParams{})
		if err != nil {
			return err
		}
		for _, c := range res.Cookies {
			storedCookies = append(storedCookies, StoredCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Expires:  float64(c.Expires),
				HTTPOnly: c.HTTPOnly,
				Secure:   c.Secure,
				SameSite: string(c.SameSite),
			})
		}
		return nil
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to extract session cookies: %w", err)
	}

	if err := saveSessionCookies(SessionFile, storedCookies); err != nil {
		fmt.Printf("%sWarning: Could not save session file: %v%s\n", ColorYellow, err, ColorReset)
	} else {
		fmt.Printf("%sLogin successful! Session state saved to %s%s\n", ColorBoldGreen, SessionFile, ColorReset)
	}

	return storedCookies, nil
}

type groupInfo struct {
	Name          string
	WeightPercent float64
}

func buildAssignmentRecord(a CanvasAssignment, sub CanvasSubmission, groupMap map[int]groupInfo) AssignmentRecord {
	pointsPossible := a.PointsPossible
	score := sub.Score
	if score == nil && sub.EnteredScore != nil {
		score = sub.EnteredScore
	}

	grade := sub.Grade
	if (grade == nil || *grade == "") && sub.EnteredGrade != nil {
		grade = sub.EnteredGrade
	}

	var percentage *float64
	if score != nil && pointsPossible != nil && *pointsPossible > 0 {
		pct := math.Round((*score / *pointsPossible)*10000) / 100
		percentage = &pct
	}

	status := "Not Submitted"
	if sub.Excused {
		status = "Excused"
	} else if sub.Missing {
		status = "Missing"
	} else if sub.WorkflowState == "graded" || score != nil {
		status = "Graded"
	} else if sub.WorkflowState == "submitted" {
		status = "Submitted (Ungraded)"
	} else if sub.Late {
		status = "Late"
	}

	grp, ok := groupMap[a.AssignmentGroupID]
	if !ok {
		grp = groupInfo{Name: "General", WeightPercent: 0.0}
	}

	return AssignmentRecord{
		ID:                 a.ID,
		Name:               a.Name,
		GroupID:            a.AssignmentGroupID,
		GroupName:          grp.Name,
		GroupWeightPercent: grp.WeightPercent,
		DueAt:              a.DueAt,
		PointsPossible:     pointsPossible,
		Score:              score,
		Grade:              grade,
		Percentage:         percentage,
		Status:             status,
		SubmittedAt:        sub.SubmittedAt,
		GradedAt:           sub.GradedAt,
		Late:               sub.Late,
		Missing:            sub.Missing,
		Excused:            sub.Excused,
		OmitFromFinalGrade: a.OmitFromFinalGrade,
		GradingType:        a.GradingType,
		HTMLURL:            a.HTMLURL,
	}
}

func isNonAffectingCategory(name string, weight float64, courseHasWeights bool) bool {
	norm := strings.ToLower(strings.TrimSpace(name))
	switch norm {
	case "not graded", "non-graded", "ungraded", "imported assignments", "migrated quizzes":
		return true
	}
	if courseHasWeights && weight <= 0 {
		return true
	}
	return false
}

func filterUngradedItems(data []CourseGradeRecord) []CourseGradeRecord {
	result := make([]CourseGradeRecord, 0, len(data))
	removedGroups := 0
	removedAsgs := 0

	for _, c := range data {
		courseHasWeights := false
		for _, g := range c.AssignmentGroups {
			if g.WeightPercent > 0 {
				courseHasWeights = true
				break
			}
		}

		var keptGroups []AssignmentGroupRecord
		for _, g := range c.AssignmentGroups {
			if isNonAffectingCategory(g.Name, g.WeightPercent, courseHasWeights) {
				removedGroups++
				continue
			}
			keptGroups = append(keptGroups, g)
		}

		var keptAsgs []AssignmentRecord
		for _, a := range c.Assignments {
			if isNonAffectingCategory(a.GroupName, a.GroupWeightPercent, courseHasWeights) ||
				a.OmitFromFinalGrade ||
				strings.EqualFold(a.GradingType, "not_graded") {
				removedAsgs++
				continue
			}
			keptAsgs = append(keptAsgs, a)
		}

		cCopy := c
		cCopy.AssignmentGroups = keptGroups
		cCopy.Assignments = keptAsgs
		result = append(result, cCopy)
	}

	if removedGroups > 0 || removedAsgs > 0 {
		fmt.Printf("%sExcluded %d ungraded categories and %d non-graded assignments (SKIP_UNGRADED_ITEMS=true).%s\n",
			ColorDim, removedGroups, removedAsgs, ColorReset)
	}
	return result
}

func parseDueTime(dueAt *string) (time.Time, bool) {
	if dueAt == nil || strings.TrimSpace(*dueAt) == "" {
		return time.Time{}, false
	}
	raw := strings.TrimSpace(*dueAt)
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func hasActiveAssignmentFilters(cfg AppConfig) bool {
	return cfg.HideFutureDue ||
		cfg.FutureOnly ||
		cfg.DueWithin > 0 ||
		cfg.DuePast > 0 ||
		cfg.HideNoDueDate ||
		cfg.MissingAfterDays >= 0 ||
		cfg.StatusFilter != "" ||
		cfg.ScoreBelow > 0 ||
		cfg.MinCategoryWeight > 0
}

func matchesStatusFilter(a AssignmentRecord, dueTime time.Time, hasDue bool, now time.Time, filterRaw string) bool {
	tokens := strings.Split(strings.ToLower(filterRaw), ",")
	st := strings.ToLower(a.Status)
	isOverdueUnsubmitted := hasDue && dueTime.Before(now) && a.Score == nil && !a.Excused && a.SubmittedAt == nil

	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		switch tok {
		case "missing", "overdue":
			if a.Missing || st == "missing" || isOverdueUnsubmitted {
				return true
			}
		case "late":
			if a.Late || st == "late" {
				return true
			}
		case "graded":
			if st == "graded" || a.Score != nil {
				return true
			}
		case "ungraded", "submitted":
			if strings.Contains(st, "submitted") {
				return true
			}
		case "excused":
			if a.Excused || st == "excused" {
				return true
			}
		default:
			if strings.Contains(st, tok) {
				return true
			}
		}
	}
	return false
}

func applyAssignmentFilters(data []CourseGradeRecord, cfg AppConfig) []CourseGradeRecord {
	if !hasActiveAssignmentFilters(cfg) {
		return data
	}

	now := time.Now()
	result := make([]CourseGradeRecord, 0, len(data))
	totalBefore := 0
	totalKept := 0

	for _, c := range data {
		var keptAsgs []AssignmentRecord
		for _, a := range c.Assignments {
			totalBefore++
			dueTime, hasDue := parseDueTime(a.DueAt)

			// 1. HideNoDueDate: exclude ungraded items with no due_at
			if cfg.HideNoDueDate && !hasDue && a.Score == nil {
				continue
			}

			// 2. HideFutureDue: exclude ungraded items due in the future
			if cfg.HideFutureDue && hasDue && dueTime.After(now) && a.Score == nil {
				continue
			}

			// 3. FutureOnly: keep ONLY items due in the future
			if cfg.FutureOnly {
				if !hasDue || !dueTime.After(now) {
					continue
				}
			}

			// 4. DueWithin: keep ONLY items due between now and now + DueWithin
			if cfg.DueWithin > 0 {
				if !hasDue || dueTime.Before(now) || dueTime.After(now.Add(cfg.DueWithin)) {
					continue
				}
			}

			// 5. DuePast: keep ONLY items due between now - DuePast and now
			if cfg.DuePast > 0 {
				if !hasDue || dueTime.After(now) || dueTime.Before(now.Add(-cfg.DuePast)) {
					continue
				}
			}

			// 6. MissingAfterDays: keep ONLY items marked Missing OR past due by >= N days without score/submission
			if cfg.MissingAfterDays >= 0 {
				threshold := time.Duration(cfg.MissingAfterDays) * 24 * time.Hour
				isUnsubmitted := a.Score == nil && !a.Excused && a.SubmittedAt == nil
				if hasDue {
					overdueBy := now.Sub(dueTime)
					if overdueBy < threshold || (!a.Missing && !isUnsubmitted) {
						continue
					}
				} else if !a.Missing {
					continue
				}
			}

			// 7. StatusFilter: match status keywords (missing, late, graded, ungraded, excused)
			if cfg.StatusFilter != "" {
				if !matchesStatusFilter(a, dueTime, hasDue, now, cfg.StatusFilter) {
					continue
				}
			}

			// 8. ScoreBelow: keep ONLY graded items with percentage < ScoreBelow
			if cfg.ScoreBelow > 0 {
				if a.Percentage == nil || *a.Percentage >= cfg.ScoreBelow {
					continue
				}
			}

			// 9. MinCategoryWeight: keep ONLY items in categories with weight >= MinCategoryWeight
			if cfg.MinCategoryWeight > 0 {
				if a.GroupWeightPercent < cfg.MinCategoryWeight {
					continue
				}
			}

			keptAsgs = append(keptAsgs, a)
			totalKept++
		}

		cCopy := c
		cCopy.Assignments = keptAsgs
		result = append(result, cCopy)
	}

	fmt.Printf("%sAssignment filters active: showing %d of %d assignments.%s\n",
		ColorBoldCyan, totalKept, totalBefore, ColorReset)
	return result
}

func letterGradeRank(g *string) int {
	if g == nil || strings.TrimSpace(*g) == "" {
		return 99
	}
	switch strings.ToUpper(strings.TrimSpace(*g)) {
	case "A+", "A":
		return 1
	case "A-":
		return 2
	case "B+":
		return 3
	case "B":
		return 4
	case "B-":
		return 5
	case "C+":
		return 6
	case "C":
		return 7
	case "C-":
		return 8
	case "D+":
		return 9
	case "D":
		return 10
	case "D-":
		return 11
	case "F":
		return 12
	default:
		return 50
	}
}

func assignmentStatusRank(a AssignmentRecord, now time.Time) int {
	dueTime, hasDue := parseDueTime(a.DueAt)
	isOverdueUnsubmitted := hasDue && dueTime.Before(now) && a.Score == nil && !a.Excused && a.SubmittedAt == nil
	st := strings.ToLower(strings.TrimSpace(a.Status))

	switch {
	case a.Missing || st == "missing" || isOverdueUnsubmitted:
		return 1 // Missing / Overdue first
	case a.Late || st == "late":
		return 2 // Late second
	case strings.Contains(st, "submitted"):
		return 3 // Submitted (Ungraded) third
	case st == "not submitted":
		return 4 // Upcoming / Not Submitted fourth
	case st == "graded":
		return 5 // Graded fifth
	case a.Excused || st == "excused":
		return 6 // Excused last
	default:
		return 7
	}
}

func applySorting(data []CourseGradeRecord, sortRaw string) []CourseGradeRecord {
	s := strings.ToLower(strings.TrimSpace(sortRaw))
	if s == "" || s == "none" || s == "default" {
		return data
	}

	now := time.Now()
	result := make([]CourseGradeRecord, len(data))
	for i, c := range data {
		cCopy := c
		asgs := make([]AssignmentRecord, len(c.Assignments))
		copy(asgs, c.Assignments)

		sort.SliceStable(asgs, func(aIdx, bIdx int) bool {
			a, b := asgs[aIdx], asgs[bIdx]
			tA, hasA := parseDueTime(a.DueAt)
			tB, hasB := parseDueTime(b.DueAt)

			switch s {
			case "date", "due", "date-asc", "due-asc":
				if hasA != hasB {
					return hasA // items with a due date come before items with no due date
				}
				if hasA && !tA.Equal(tB) {
					return tA.Before(tB)
				}
				return a.Name < b.Name

			case "date-desc", "due-desc":
				if hasA != hasB {
					return hasA
				}
				if hasA && !tA.Equal(tB) {
					return tA.After(tB)
				}
				return a.Name < b.Name

			case "status", "status-asc":
				rA := assignmentStatusRank(a, now)
				rB := assignmentStatusRank(b, now)
				if rA != rB {
					return rA < rB
				}
				if hasA != hasB {
					return hasA
				}
				if hasA && !tA.Equal(tB) {
					return tA.Before(tB)
				}
				return a.Name < b.Name

			case "status-desc":
				rA := assignmentStatusRank(a, now)
				rB := assignmentStatusRank(b, now)
				if rA != rB {
					return rA > rB
				}
				return a.Name < b.Name

			case "score", "score-asc", "grade-desc":
				hasPctA := a.Percentage != nil
				hasPctB := b.Percentage != nil
				if hasPctA != hasPctB {
					return hasPctA // graded items with a score come before ungraded
				}
				if hasPctA && *a.Percentage != *b.Percentage {
					return *a.Percentage < *b.Percentage
				}
				return a.Name < b.Name

			case "score-desc", "grade", "grade-asc":
				hasPctA := a.Percentage != nil
				hasPctB := b.Percentage != nil
				if hasPctA != hasPctB {
					return hasPctA
				}
				if hasPctA && *a.Percentage != *b.Percentage {
					return *a.Percentage > *b.Percentage
				}
				return a.Name < b.Name

			case "weight", "weight-desc":
				if a.GroupWeightPercent != b.GroupWeightPercent {
					return a.GroupWeightPercent > b.GroupWeightPercent
				}
				if hasA != hasB {
					return hasA
				}
				if hasA && !tA.Equal(tB) {
					return tA.Before(tB)
				}
				return a.Name < b.Name

			case "weight-asc":
				if a.GroupWeightPercent != b.GroupWeightPercent {
					return a.GroupWeightPercent < b.GroupWeightPercent
				}
				return a.Name < b.Name

			default:
				return a.Name < b.Name
			}
		})

		cCopy.Assignments = asgs
		result[i] = cCopy
	}

	// Also sort courses in the overview table when sorting by score or grade
	switch s {
	case "score", "score-asc", "grade-desc":
		sort.SliceStable(result, func(i, j int) bool {
			if result[i].StudentName != result[j].StudentName {
				return result[i].StudentName < result[j].StudentName
			}
			hasI := result[i].OverallScore != nil
			hasJ := result[j].OverallScore != nil
			if hasI != hasJ {
				return hasI
			}
			if hasI && *result[i].OverallScore != *result[j].OverallScore {
				return *result[i].OverallScore < *result[j].OverallScore
			}
			return result[i].CourseName < result[j].CourseName
		})
	case "score-desc", "grade", "grade-asc":
		sort.SliceStable(result, func(i, j int) bool {
			if result[i].StudentName != result[j].StudentName {
				return result[i].StudentName < result[j].StudentName
			}
			rI := letterGradeRank(result[i].OverallGrade)
			rJ := letterGradeRank(result[j].OverallGrade)
			if s == "grade" || s == "grade-asc" {
				if rI != rJ {
					return rI < rJ
				}
			}
			hasI := result[i].OverallScore != nil
			hasJ := result[j].OverallScore != nil
			if hasI != hasJ {
				return hasI
			}
			if hasI && *result[i].OverallScore != *result[j].OverallScore {
				return *result[i].OverallScore > *result[j].OverallScore
			}
			return result[i].CourseName < result[j].CourseName
		})
	}

	fmt.Printf("%sSorted results by: %s%s\n", ColorBoldCyan, s, ColorReset)
	return result
}

func extractOverallScoreAndGrade(stEnr CanvasEnrollment) (*float64, *string) {
	overallScore := stEnr.CurrentPeriodComputedCurrentScore
	if overallScore == nil {
		overallScore = stEnr.ComputedCurrentScore
	}
	if overallScore == nil {
		overallScore = stEnr.CurrentScore
	}

	overallGrade := stEnr.CurrentPeriodComputedCurrentGrade
	if overallGrade == nil || *overallGrade == "" {
		overallGrade = stEnr.ComputedCurrentGrade
	}
	if overallGrade == nil || *overallGrade == "" {
		overallGrade = stEnr.ComputedCurrentLetterGrade
	}
	if overallGrade == nil || *overallGrade == "" {
		overallGrade = stEnr.CurrentGrade
	}
	return overallScore, overallGrade
}

func scrapeCanvasData(client *CanvasClient, skipNonGraded bool) ([]CourseGradeRecord, error) {
	var selfUser CanvasUser
	_ = client.getJSON("/api/v1/users/self", &selfUser)

	var observees []CanvasUser
	_ = client.getJSON("/api/v1/users/self/observees?per_page=100", &observees)

	studentMap := make(map[int]string)
	if selfUser.ID != 0 {
		name := selfUser.Name
		if name == "" {
			name = "Self"
		}
		studentMap[selfUser.ID] = name
	}
	var obsNames []string
	for _, obs := range observees {
		if obs.ID != 0 {
			name := obs.Name
			if name == "" {
				name = fmt.Sprintf("Student %d", obs.ID)
			}
			studentMap[obs.ID] = name
			obsNames = append(obsNames, name)
		}
	}

	if len(obsNames) > 0 {
		fmt.Printf("%sDetected Parent/Observer account observing: %s%s%s\n", ColorCyan, ColorBold, strings.Join(obsNames, ", "), ColorReset)
	}

	var courses []CanvasCourse
	err := client.getJSON("/api/v1/courses?enrollment_state=active&include[]=total_scores&include[]=observed_users&include[]=current_grading_period_scores&include[]=term&per_page=100", &courses)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch courses: %w", err)
	}

	var validCourses []CanvasCourse
	for _, c := range courses {
		if c.ID != 0 && c.Name != "" && !c.AccessRestrictedByDate {
			validCourses = append(validCourses, c)
		}
	}

	fmt.Printf("%sFound %d active courses.%s\n", ColorCyan, len(validCourses), ColorReset)

	var fullResults []CourseGradeRecord
	skippedCount := 0
	fetchedCount := 0

	for _, course := range validCourses {
		courseID := course.ID
		courseName := course.Name

		var studentEnrollments []CanvasEnrollment
		for _, e := range course.Enrollments {
			if e.Type == "student" {
				studentEnrollments = append(studentEnrollments, e)
			}
		}
		if len(studentEnrollments) == 0 {
			for _, e := range course.Enrollments {
				uid := e.AssociatedUserID
				if uid == 0 {
					uid = e.UserID
				}
				studentEnrollments = append(studentEnrollments, CanvasEnrollment{
					UserID: uid,
					Type:   e.Type,
				})
			}
		}

		// B. Skip non-graded courses if SKIP_NONGRADED=true
		if skipNonGraded {
			var gradedEnrollments []CanvasEnrollment
			for _, stEnr := range studentEnrollments {
				sc, gr := extractOverallScoreAndGrade(stEnr)
				if sc != nil || (gr != nil && *gr != "") {
					gradedEnrollments = append(gradedEnrollments, stEnr)
				}
			}
			if len(gradedEnrollments) == 0 {
				skippedCount++
				continue
			}
			studentEnrollments = gradedEnrollments
		}

		// D. Polite pacing delay with jitter (150ms - 300ms) between course fetches
		if fetchedCount > 0 {
			jitterMs := 150 + rand.Intn(151)
			time.Sleep(time.Duration(jitterMs) * time.Millisecond)
		}
		fetchedCount++

		// A. Single combined call for assignment_groups AND assignments
		var rawGroups []CanvasAssignmentGroup
		_ = client.getJSON(fmt.Sprintf("/api/v1/courses/%d/assignment_groups?include[]=assignments&per_page=100", courseID), &rawGroups)

		var rawAssignments []CanvasAssignment
		var groupsData []AssignmentGroupRecord
		groupMap := make(map[int]groupInfo)
		for _, g := range rawGroups {
			gname := g.Name
			if gname == "" {
				gname = "Unnamed Group"
			}
			rules := g.Rules
			if rules == nil {
				rules = map[string]any{}
			}
			groupMap[g.ID] = groupInfo{Name: gname, WeightPercent: g.GroupWeight}
			groupsData = append(groupsData, AssignmentGroupRecord{
				ID:            g.ID,
				Name:          gname,
				WeightPercent: g.GroupWeight,
				Rules:         rules,
			})
			rawAssignments = append(rawAssignments, g.Assignments...)
		}

		seenStudents := make(map[int]bool)
		for _, stEnr := range studentEnrollments {
			studentID := stEnr.UserID
			if seenStudents[studentID] {
				continue
			}
			seenStudents[studentID] = true

			studentName, ok := studentMap[studentID]
			if !ok {
				if studentID != 0 {
					studentName = fmt.Sprintf("Student %d", studentID)
				} else {
					studentName = "Student"
				}
			}

			overallScore, overallGrade := extractOverallScoreAndGrade(stEnr)

			finalScore := stEnr.CurrentPeriodComputedFinalScore
			if finalScore == nil {
				finalScore = stEnr.ComputedFinalScore
			}

			subMap := make(map[int]CanvasSubmission)
			if studentID != 0 {
				var subs []CanvasSubmission
				subEndpoint := fmt.Sprintf("/api/v1/courses/%d/students/submissions?student_ids[]=%d&per_page=100", courseID, studentID)
				if err := client.getJSON(subEndpoint, &subs); err == nil {
					for _, s := range subs {
						if s.AssignmentID != 0 {
							subMap[s.AssignmentID] = s
						}
					}
				}
			}

			assignmentsData := make([]AssignmentRecord, 0, len(rawAssignments))
			for _, a := range rawAssignments {
				sub, hasSub := subMap[a.ID]
				if !hasSub && a.Submission != nil {
					sub = *a.Submission
				}
				assignmentsData = append(assignmentsData, buildAssignmentRecord(a, sub, groupMap))
			}

			fullResults = append(fullResults, CourseGradeRecord{
				StudentID:        studentID,
				StudentName:      studentName,
				CourseID:         courseID,
				CourseName:       courseName,
				CourseCode:       course.CourseCode,
				GradingPeriod:    stEnr.CurrentGradingPeriodTitle,
				OverallScore:     overallScore,
				OverallGrade:     overallGrade,
				FinalScore:       finalScore,
				AssignmentGroups: groupsData,
				Assignments:      assignmentsData,
			})
		}
	}

	if skippedCount > 0 {
		fmt.Printf("%sSkipped %d non-graded courses (SKIP_NONGRADED=true).%s\n", ColorDim, skippedCount, ColorReset)
	}

	sort.Slice(fullResults, func(i, j int) bool {
		if fullResults[i].StudentName != fullResults[j].StudentName {
			return fullResults[i].StudentName < fullResults[j].StudentName
		}
		return fullResults[i].CourseName < fullResults[j].CourseName
	})

	return fullResults, nil
}

func formatFloatTrim(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func exportData(outputDir string, data []CourseGradeRecord) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// 1. Export JSON
	jsonPath := filepath.Join(outputDir, "canvas_grades.json")
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		return err
	}
	fmt.Printf("%sSaved detailed JSON data to: %s%s\n", ColorBoldGreen, jsonPath, ColorReset)

	// 2. Export Courses Summary CSV
	coursesCSV := filepath.Join(outputDir, "courses_summary.csv")
	cf, err := os.Create(coursesCSV)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(cf)
	_ = cw.Write([]string{
		"Student Name",
		"Course ID",
		"Course Name",
		"Course Code",
		"Grading Period",
		"Current Score (%)",
		"Current Grade",
	})
	for _, c := range data {
		gp := ""
		if c.GradingPeriod != nil {
			gp = *c.GradingPeriod
		}
		scoreStr := "N/A"
		if c.OverallScore != nil {
			scoreStr = fmt.Sprintf("%s%%", formatFloatTrim(*c.OverallScore))
		}
		gradeStr := "N/A"
		if c.OverallGrade != nil && *c.OverallGrade != "" {
			gradeStr = *c.OverallGrade
		}
		_ = cw.Write([]string{
			c.StudentName,
			strconv.Itoa(c.CourseID),
			c.CourseName,
			c.CourseCode,
			gp,
			scoreStr,
			gradeStr,
		})
	}
	cw.Flush()
	cf.Close()
	fmt.Printf("%sSaved courses summary to: %s%s\n", ColorBoldGreen, coursesCSV, ColorReset)

	// 3. Export Assignment Weights CSV
	weightsCSV := filepath.Join(outputDir, "assignment_weights.csv")
	wf, err := os.Create(weightsCSV)
	if err != nil {
		return err
	}
	ww := csv.NewWriter(wf)
	_ = ww.Write([]string{"Student Name", "Course ID", "Course Name", "Group / Category", "Weight (%)"})
	for _, c := range data {
		for _, g := range c.AssignmentGroups {
			_ = ww.Write([]string{
				c.StudentName,
				strconv.Itoa(c.CourseID),
				c.CourseName,
				g.Name,
				fmt.Sprintf("%s%%", formatFloatTrim(g.WeightPercent)),
			})
		}
	}
	ww.Flush()
	wf.Close()
	fmt.Printf("%sSaved assignment weights to: %s%s\n", ColorBoldGreen, weightsCSV, ColorReset)

	// 4. Export Assignments Detail CSV
	assignmentsCSV := filepath.Join(outputDir, "assignments_detail.csv")
	af, err := os.Create(assignmentsCSV)
	if err != nil {
		return err
	}
	aw := csv.NewWriter(af)
	_ = aw.Write([]string{
		"Student Name",
		"Course Name",
		"Assignment Name",
		"Category (Weight Group)",
		"Category Weight (%)",
		"Score Earned",
		"Points Possible",
		"Score (%)",
		"Grade",
		"Status",
		"Due Date",
		"Graded At",
	})
	for _, c := range data {
		for _, a := range c.Assignments {
			scoreStr := ""
			if a.Score != nil {
				scoreStr = formatFloatTrim(*a.Score)
			}
			ptsStr := ""
			if a.PointsPossible != nil {
				ptsStr = formatFloatTrim(*a.PointsPossible)
			}
			pctStr := ""
			if a.Percentage != nil {
				pctStr = fmt.Sprintf("%s%%", formatFloatTrim(*a.Percentage))
			}
			gradeStr := ""
			if a.Grade != nil {
				gradeStr = *a.Grade
			}
			dueStr := ""
			if a.DueAt != nil {
				dueStr = *a.DueAt
			}
			gradedStr := ""
			if a.GradedAt != nil {
				gradedStr = *a.GradedAt
			}
			_ = aw.Write([]string{
				c.StudentName,
				c.CourseName,
				a.Name,
				a.GroupName,
				fmt.Sprintf("%s%%", formatFloatTrim(a.GroupWeightPercent)),
				scoreStr,
				ptsStr,
				pctStr,
				gradeStr,
				a.Status,
				dueStr,
				gradedStr,
			})
		}
	}
	aw.Flush()
	af.Close()
	fmt.Printf("%sSaved full assignments list to: %s%s\n", ColorBoldGreen, assignmentsCSV, ColorReset)
	return nil
}

func openInBrowser(filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err == nil {
		filePath = absPath
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", filePath)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", filePath)
	default:
		cmd = exec.Command("xdg-open", filePath)
	}
	return cmd.Start()
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func exportMarkdown(outputDir string, data []CourseGradeRecord, cacheNotice string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	mdPath := filepath.Join(outputDir, "canvas_grades.md")

	var sb strings.Builder
	now := time.Now()

	totalAsgs := 0
	missingCount := 0
	lateCount := 0
	upcomingCount := 0
	studentSet := make(map[string]bool)

	for _, c := range data {
		studentSet[c.StudentName] = true
		for _, a := range c.Assignments {
			totalAsgs++
			dueTime, hasDue := parseDueTime(a.DueAt)
			isOverdueUnsubmitted := hasDue && dueTime.Before(now) && a.Score == nil && !a.Excused && a.SubmittedAt == nil
			if a.Missing || strings.EqualFold(a.Status, "Missing") || isOverdueUnsubmitted {
				missingCount++
			}
			if a.Late || strings.EqualFold(a.Status, "Late") {
				lateCount++
			}
			if hasDue && dueTime.After(now) {
				upcomingCount++
			}
		}
	}

	var students []string
	for st := range studentSet {
		students = append(students, st)
	}
	sort.Strings(students)

	sb.WriteString("# Canvas LMS Gradebook & Assignment Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Generated At**: `%s`\n", now.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Students**: %s\n", strings.Join(students, ", ")))
	sb.WriteString(fmt.Sprintf("- **Total Courses**: %d\n", len(data)))
	sb.WriteString(fmt.Sprintf("- **Total Assignments Shown**: %d (Missing/Overdue: %d | Late: %d | Upcoming Future: %d)\n",
		totalAsgs, missingCount, lateCount, upcomingCount))
	if cacheNotice != "" {
		sb.WriteString(fmt.Sprintf("- **Data Source**: Cached (`%s`)\n", cacheNotice))
	} else {
		sb.WriteString("- **Data Source**: Live Canvas API\n")
	}
	sb.WriteString("\n## 1. Courses Overview\n\n")
	sb.WriteString("| Student | Course Name | Course ID | Score (%) | Letter Grade | Assignments | Weight Categories |\n")
	sb.WriteString("| :--- | :--- | :---: | :---: | :---: | :---: | :--- |\n")

	for _, c := range data {
		scoreStr := "N/A"
		if c.OverallScore != nil {
			scoreStr = fmt.Sprintf("%.2f%%", *c.OverallScore)
		}
		gradeStr := "-"
		if c.OverallGrade != nil && *c.OverallGrade != "" {
			gradeStr = *c.OverallGrade
		}

		var weights []string
		seenW := make(map[string]bool)
		for _, g := range c.AssignmentGroups {
			if g.WeightPercent > 0 {
				lbl := fmt.Sprintf("%s (%s%%)", g.Name, formatFloatTrim(g.WeightPercent))
				if !seenW[lbl] {
					seenW[lbl] = true
					weights = append(weights, lbl)
				}
			}
		}
		weightStr := "Unweighted / Points"
		if len(weights) > 0 {
			weightStr = strings.Join(weights, ", ")
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s | %d | %s |\n",
			mdEscape(c.StudentName),
			mdEscape(c.CourseName),
			c.CourseID,
			scoreStr,
			mdEscape(gradeStr),
			len(c.Assignments),
			mdEscape(weightStr),
		))
	}

	// Action Items Section for AI quick consumption
	var actionItems []string
	for _, c := range data {
		for _, a := range c.Assignments {
			dueTime, hasDue := parseDueTime(a.DueAt)
			isOverdueUnsubmitted := hasDue && dueTime.Before(now) && a.Score == nil && !a.Excused && a.SubmittedAt == nil
			if a.Missing || a.Late || isOverdueUnsubmitted {
				dueDisp := "No Due Date"
				if a.DueAt != nil && len(*a.DueAt) >= 10 {
					dueDisp = (*a.DueAt)[:10]
				}
				ptsDisp := "-"
				if a.Score != nil && a.PointsPossible != nil {
					ptsDisp = fmt.Sprintf("%s/%s", formatFloatTrim(*a.Score), formatFloatTrim(*a.PointsPossible))
				} else if a.PointsPossible != nil {
					ptsDisp = fmt.Sprintf("-/%s", formatFloatTrim(*a.PointsPossible))
				}
				flagTag := a.Status
				if isOverdueUnsubmitted && !a.Missing {
					flagTag = "Overdue (Not Submitted)"
				} else if a.Late && a.Status == "Graded" {
					flagTag = "Graded (Submitted Late)"
				} else if a.Late && !strings.Contains(strings.ToLower(a.Status), "late") {
					flagTag = a.Status + " (Late)"
				}
				actionItems = append(actionItems, fmt.Sprintf("- **%s** | **%s** — `%s` (%s, Weight: %s%%) | Score: `%s` | Status: **%s** | Due: `%s`",
					mdEscape(c.StudentName),
					mdEscape(c.CourseName),
					mdEscape(a.Name),
					mdEscape(a.GroupName),
					formatFloatTrim(a.GroupWeightPercent),
					ptsDisp,
					flagTag,
					dueDisp,
				))
			}
		}
	}

	if len(actionItems) > 0 {
		sb.WriteString("\n## 2. Action Items (Missing, Overdue, or Late Assignments)\n\n")
		for _, item := range actionItems {
			sb.WriteString(item + "\n")
		}
	}

	sb.WriteString("\n## 3. Course & Assignment Breakdown\n\n")
	for _, c := range data {
		if len(c.Assignments) == 0 && c.OverallScore == nil {
			continue
		}
		scoreHdr := "N/A"
		if c.OverallScore != nil {
			scoreHdr = fmt.Sprintf("%.2f%%", *c.OverallScore)
		}
		gradeHdr := "-"
		if c.OverallGrade != nil && *c.OverallGrade != "" {
			gradeHdr = *c.OverallGrade
		}

		sb.WriteString(fmt.Sprintf("### %s — %s (Current: %s | Grade: %s)\n\n",
			mdEscape(c.StudentName), mdEscape(c.CourseName), scoreHdr, mdEscape(gradeHdr)))

		if len(c.Assignments) == 0 {
			sb.WriteString("_No assignments matching active filters._\n\n")
			continue
		}

		sb.WriteString("| Assignment | Category (Weight) | Score | Percent | Status | Due Date |\n")
		sb.WriteString("| :--- | :--- | :---: | :---: | :---: | :---: |\n")

		for _, a := range c.Assignments {
			scoreDisp := "-"
			if a.Score != nil && a.PointsPossible != nil {
				scoreDisp = fmt.Sprintf("%s/%s", formatFloatTrim(*a.Score), formatFloatTrim(*a.PointsPossible))
			} else if a.Score != nil {
				scoreDisp = formatFloatTrim(*a.Score)
			} else if a.PointsPossible != nil {
				scoreDisp = fmt.Sprintf("-/%s", formatFloatTrim(*a.PointsPossible))
			}

			pctDisp := "-"
			if a.Percentage != nil {
				pctDisp = fmt.Sprintf("%.1f%%", *a.Percentage)
			}

			catDisp := a.GroupName
			if a.GroupWeightPercent > 0 {
				catDisp = fmt.Sprintf("%s (%s%%)", a.GroupName, formatFloatTrim(a.GroupWeightPercent))
			}

			dueDisp := "-"
			if a.DueAt != nil && len(*a.DueAt) >= 10 {
				dueDisp = (*a.DueAt)[:10]
			}

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				mdEscape(a.Name),
				mdEscape(catDisp),
				scoreDisp,
				pctDisp,
				mdEscape(a.Status),
				dueDisp,
			))
		}
		sb.WriteString("\n")
	}

	if err := os.WriteFile(mdPath, []byte(sb.String()), 0644); err != nil {
		return "", err
	}
	fmt.Printf("%sSaved AI-ready Markdown report to: %s%s\n", ColorBoldGreen, mdPath, ColorReset)
	return mdPath, nil
}

func exportHTML(outputDir string, data []CourseGradeRecord, cacheNotice string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	htmlPath := filepath.Join(outputDir, "canvas_grades.html")
	now := time.Now()

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Canvas Gradebook Report</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f8fafc;
      --surface: #ffffff;
      --surface-subtle: #f1f5f9;
      --border: #e2e8f0;
      --text: #0f172a;
      --text-muted: #64748b;
      --accent: #2563eb;
      --green-bg: #dcfce7;
      --green-fg: #166534;
      --yellow-bg: #fef9c3;
      --yellow-fg: #854d0e;
      --red-bg: #fee2e2;
      --red-fg: #991b1b;
      --cyan-bg: #e0f2fe;
      --cyan-fg: #075985;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #0b0f19;
        --surface: #111827;
        --surface-subtle: #1e293b;
        --border: #334155;
        --text: #f8fafc;
        --text-muted: #94a3b8;
        --accent: #60a5fa;
        --green-bg: rgba(22, 101, 52, 0.35);
        --green-fg: #86efac;
        --yellow-bg: rgba(133, 77, 14, 0.35);
        --yellow-fg: #fde047;
        --red-bg: rgba(153, 27, 27, 0.4);
        --red-fg: #fca5a5;
        --cyan-bg: rgba(7, 89, 133, 0.35);
        --cyan-fg: #7dd3fc;
      }
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      padding: 2rem 1.25rem 4rem;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Inter, sans-serif;
      background: var(--bg);
      color: var(--text);
      line-height: 1.5;
    }
    .container {
      max-width: 1240px;
      margin: 0 auto;
    }
    header {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      align-items: flex-end;
      gap: 1rem;
      margin-bottom: 1.5rem;
      padding-bottom: 1rem;
      border-bottom: 1px solid var(--border);
    }
    h1 {
      margin: 0;
      font-size: 1.75rem;
      letter-spacing: -0.02em;
    }
    .meta {
      color: var(--text-muted);
      font-size: 0.9rem;
    }
    .cache-banner {
      background: var(--yellow-bg);
      color: var(--yellow-fg);
      border: 1px solid var(--border);
      padding: 0.75rem 1rem;
      border-radius: 0.5rem;
      font-weight: 600;
      margin-bottom: 1.5rem;
      font-size: 0.92rem;
    }
    .card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 0.75rem;
      margin-bottom: 1.5rem;
      overflow: hidden;
      box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
    }
    .card-header {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      align-items: center;
      gap: 0.75rem;
      padding: 0.9rem 1.25rem;
      background: var(--surface-subtle);
      border-bottom: 1px solid var(--border);
    }
    .card-title {
      margin: 0;
      font-size: 1.05rem;
      font-weight: 600;
    }
    .badge {
      display: inline-block;
      padding: 0.18rem 0.55rem;
      border-radius: 9999px;
      font-size: 0.78rem;
      font-weight: 600;
      white-space: nowrap;
    }
    .badge-green { background: var(--green-bg); color: var(--green-fg); }
    .badge-yellow { background: var(--yellow-bg); color: var(--yellow-fg); }
    .badge-red { background: var(--red-bg); color: var(--red-fg); }
    .badge-cyan { background: var(--cyan-bg); color: var(--cyan-fg); }
    .table-wrapper {
      overflow: auto;
      max-inline-size: 100%;
      max-block-size: min(560px, 80vh);
      container-type: inline-size;
    }
    table {
      width: 100%;
      border-collapse: separate;
      border-spacing: 0;
      font-size: 0.9rem;
    }
    caption {
      position: absolute;
      width: 1px;
      height: 1px;
      padding: 0;
      margin: -1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
      border: 0;
    }
    thead {
      position: sticky;
      inset-block-start: 0;
      z-index: 2;
    }
    th, td {
      padding: 0.65rem 1rem;
      text-align: left;
      border-bottom: 1px solid var(--border);
    }
    thead th {
      background: var(--surface-subtle);
      color: var(--text-muted);
      font-weight: 600;
      font-size: 0.8rem;
      text-transform: uppercase;
      letter-spacing: 0.03em;
    }
    tbody th {
      position: sticky;
      inset-inline-start: 0;
      z-index: 1;
      background: var(--surface);
      font-weight: 600;
    }
    tbody tr:hover td,
    tbody tr:hover th {
      background: var(--surface-subtle);
    }
    .num {
      text-align: right;
      font-variant-numeric: tabular-nums;
    }
    a.asg-link {
      color: inherit;
      text-decoration: none;
    }
    a.asg-link:hover {
      color: var(--accent);
      text-decoration: underline;
    }
    @container (width < 640px) {
      table, thead, tbody, tr, th, td {
        display: block;
      }
      thead {
        display: none;
      }
      tr {
        margin: 0.75rem;
        border: 1px solid var(--border);
        border-radius: 0.5rem;
        overflow: hidden;
      }
      tbody th {
        position: sticky;
        inset-block-start: 0;
        z-index: 2;
        background: var(--surface-subtle);
      }
      td {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 0.75rem;
        text-align: right;
      }
      td:nth-child(2)::before { content: var(--label-2) ": "; font-weight: 600; color: var(--text-muted); text-align: left; }
      td:nth-child(3)::before { content: var(--label-3) ": "; font-weight: 600; color: var(--text-muted); text-align: left; }
      td:nth-child(4)::before { content: var(--label-4) ": "; font-weight: 600; color: var(--text-muted); text-align: left; }
      td:nth-child(5)::before { content: var(--label-5) ": "; font-weight: 600; color: var(--text-muted); text-align: left; }
      td:nth-child(6)::before { content: var(--label-6) ": "; font-weight: 600; color: var(--text-muted); text-align: left; }
    }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div>
        <h1>Canvas Gradebook Overview</h1>
        <div class="meta">Generated `)
	sb.WriteString(html.EscapeString(now.Format("Jan 2, 2006 at 3:04:05 PM")))
	sb.WriteString(`</div>
      </div>
    </header>
`)

	if cacheNotice != "" {
		sb.WriteString(fmt.Sprintf(`    <div class="cache-banner">⚡ [USING CACHED RESULTS] %s</div>`+"\n", html.EscapeString(cacheNotice)))
	}

	// Overview Card
	sb.WriteString(`    <section class="card">
      <div class="card-header">
        <h2 class="card-title">Courses Summary</h2>
      </div>
      <div tabindex="0" role="region" aria-labelledby="overview-caption" class="table-wrapper">
        <table style="--label-2: 'Course'; --label-3: 'Score'; --label-4: 'Grade'; --label-5: 'Assignments'; --label-6: 'Weight Categories';">
          <caption id="overview-caption">Canvas Gradebook Overview across students and courses</caption>
          <thead>
            <tr>
              <th scope="col">Student</th>
              <th scope="col">Course Name</th>
              <th scope="col" class="num">Score</th>
              <th scope="col">Grade</th>
              <th scope="col" class="num">Asgs</th>
              <th scope="col">Weight Categories</th>
            </tr>
          </thead>
          <tbody>
`)

	for i, c := range data {
		scoreStr := "N/A"
		if c.OverallScore != nil {
			scoreStr = fmt.Sprintf("%.2f%%", *c.OverallScore)
		}
		gradeStr := "-"
		if c.OverallGrade != nil && *c.OverallGrade != "" {
			gradeStr = *c.OverallGrade
		}

		var weights []string
		seenW := make(map[string]bool)
		for _, g := range c.AssignmentGroups {
			if g.WeightPercent > 0 {
				lbl := fmt.Sprintf("%s (%s%%)", g.Name, formatFloatTrim(g.WeightPercent))
				if !seenW[lbl] {
					seenW[lbl] = true
					weights = append(weights, lbl)
				}
			}
		}
		weightStr := "Unweighted / Points"
		if len(weights) > 0 {
			weightStr = strings.Join(weights, ", ")
		}

		gradeBadge := "badge-cyan"
		if c.OverallScore != nil {
			switch {
			case *c.OverallScore >= 90:
				gradeBadge = "badge-green"
			case *c.OverallScore >= 80:
				gradeBadge = "badge-cyan"
			case *c.OverallScore >= 70:
				gradeBadge = "badge-yellow"
			default:
				gradeBadge = "badge-red"
			}
		}

		sb.WriteString(fmt.Sprintf(`            <tr>
              <th scope="row">%s</th>
              <td><a class="asg-link" href="#course-%d">%s</a></td>
              <td class="num"><strong>%s</strong></td>
              <td><span class="badge %s">%s</span></td>
              <td class="num">%d</td>
              <td>%s</td>
            </tr>
`,
			html.EscapeString(c.StudentName),
			i,
			html.EscapeString(c.CourseName),
			html.EscapeString(scoreStr),
			gradeBadge,
			html.EscapeString(gradeStr),
			len(c.Assignments),
			html.EscapeString(weightStr),
		))
	}

	sb.WriteString(`          </tbody>
        </table>
      </div>
    </section>
`)

	// Per-Course Assignment Cards
	for i, c := range data {
		if len(c.Assignments) == 0 && c.OverallScore == nil {
			continue
		}
		scoreHdr := "N/A"
		if c.OverallScore != nil {
			scoreHdr = fmt.Sprintf("%.2f%%", *c.OverallScore)
		}
		gradeHdr := "-"
		if c.OverallGrade != nil && *c.OverallGrade != "" {
			gradeHdr = *c.OverallGrade
		}
		captionID := fmt.Sprintf("course-caption-%d", i)

		sb.WriteString(fmt.Sprintf(`    <section class="card" id="course-%d">
      <div class="card-header">
        <h2 class="card-title">%s — %s</h2>
        <div>
          <span class="badge badge-cyan">Current: %s</span>
          <span class="badge badge-green">Grade: %s</span>
        </div>
      </div>
`, i, html.EscapeString(c.StudentName), html.EscapeString(c.CourseName), html.EscapeString(scoreHdr), html.EscapeString(gradeHdr)))

		if len(c.Assignments) == 0 {
			sb.WriteString(`      <div style="padding: 1rem 1.25rem; color: var(--text-muted);">No assignments matching active filters.</div>
    </section>
`)
			continue
		}

		sb.WriteString(fmt.Sprintf(`      <div tabindex="0" role="region" aria-labelledby="%s" class="table-wrapper">
        <table style="--label-2: 'Category (Weight)'; --label-3: 'Score'; --label-4: 'Percent'; --label-5: 'Status'; --label-6: 'Due Date';">
          <caption id="%s">%s — %s Assignments</caption>
          <thead>
            <tr>
              <th scope="col">Assignment</th>
              <th scope="col">Category (Weight)</th>
              <th scope="col" class="num">Score</th>
              <th scope="col" class="num">Percent</th>
              <th scope="col">Status</th>
              <th scope="col">Due Date</th>
            </tr>
          </thead>
          <tbody>
`, captionID, captionID, html.EscapeString(c.StudentName), html.EscapeString(c.CourseName)))

		for _, a := range c.Assignments {
			scoreDisp := "-"
			if a.Score != nil && a.PointsPossible != nil {
				scoreDisp = fmt.Sprintf("%s/%s", formatFloatTrim(*a.Score), formatFloatTrim(*a.PointsPossible))
			} else if a.Score != nil {
				scoreDisp = formatFloatTrim(*a.Score)
			} else if a.PointsPossible != nil {
				scoreDisp = fmt.Sprintf("-/%s", formatFloatTrim(*a.PointsPossible))
			}

			pctDisp := "-"
			if a.Percentage != nil {
				pctDisp = fmt.Sprintf("%.1f%%", *a.Percentage)
			}

			catDisp := a.GroupName
			if a.GroupWeightPercent > 0 {
				catDisp = fmt.Sprintf("%s (%s%%)", a.GroupName, formatFloatTrim(a.GroupWeightPercent))
			}

			dueDisp := "-"
			if a.DueAt != nil && len(*a.DueAt) >= 10 {
				dueDisp = (*a.DueAt)[:10]
			}

			statusBadge := "badge-yellow"
			switch a.Status {
			case "Graded":
				statusBadge = "badge-green"
			case "Missing":
				statusBadge = "badge-red"
			case "Excused":
				statusBadge = "badge-cyan"
			}

			nameCell := html.EscapeString(a.Name)
			if a.HTMLURL != nil && *a.HTMLURL != "" {
				nameCell = fmt.Sprintf(`<a class="asg-link" href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
					html.EscapeString(*a.HTMLURL), html.EscapeString(a.Name))
			}

			sb.WriteString(fmt.Sprintf(`            <tr>
              <th scope="row">%s</th>
              <td>%s</td>
              <td class="num">%s</td>
              <td class="num">%s</td>
              <td><span class="badge %s">%s</span></td>
              <td>%s</td>
            </tr>
`,
				nameCell,
				html.EscapeString(catDisp),
				html.EscapeString(scoreDisp),
				html.EscapeString(pctDisp),
				statusBadge,
				html.EscapeString(a.Status),
				html.EscapeString(dueDisp),
			))
		}

		sb.WriteString(`          </tbody>
        </table>
      </div>
    </section>
`)
	}

	sb.WriteString(`  </div>
</body>
</html>
`)

	if err := os.WriteFile(htmlPath, []byte(sb.String()), 0644); err != nil {
		return "", err
	}
	fmt.Printf("%sSaved HTML report to: %s%s\n", ColorBoldGreen, htmlPath, ColorReset)
	return htmlPath, nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func displayTerminalSummary(data []CourseGradeRecord, cacheNotice string, filterActive bool) {
	fmt.Println()
	if cacheNotice != "" {
		fmt.Printf("%s⚡ [USING CACHED RESULTS] %s%s\n", ColorBoldYellow, cacheNotice, ColorReset)
	}
	fmt.Printf("%s────────────────────────────── Canvas Gradebook Overview ──────────────────────────────%s\n", ColorBoldMagenta, ColorReset)
	fmt.Printf("%-16s │ %-26s │ %10s │ %5s │ %5s │ %s\n", "Student", "Course Name", "Score", "Grade", "Asgs", "Weight Categories")
	fmt.Println(strings.Repeat("─", 110))

	for _, c := range data {
		scoreStr := "N/A"
		if c.OverallScore != nil {
			scoreStr = fmt.Sprintf("%.2f%%", *c.OverallScore)
		}
		gradeStr := "-"
		if c.OverallGrade != nil && *c.OverallGrade != "" {
			gradeStr = *c.OverallGrade
		}

		var weights []string
		seenW := make(map[string]bool)
		for _, g := range c.AssignmentGroups {
			if g.WeightPercent > 0 {
				lbl := fmt.Sprintf("%s (%s%%)", g.Name, formatFloatTrim(g.WeightPercent))
				if !seenW[lbl] {
					seenW[lbl] = true
					weights = append(weights, lbl)
				}
			}
		}
		weightStr := "Unweighted / Points"
		if len(weights) > 0 {
			weightStr = strings.Join(weights, ", ")
		}

		fmt.Printf("%s%-16s%s │ %s%-26s%s │ %s%10s%s │ %s%5s%s │ %5d │ %s\n",
			ColorBoldCyan, truncate(c.StudentName, 16), ColorReset,
			ColorBoldWhite, truncate(c.CourseName, 26), ColorReset,
			ColorBoldGreen, scoreStr, ColorReset,
			ColorBoldYellow, gradeStr, ColorReset,
			len(c.Assignments),
			truncate(weightStr, 50),
		)
	}
	fmt.Println(strings.Repeat("─", 110))
	fmt.Println()

	for _, c := range data {
		if len(c.Assignments) == 0 && (c.OverallScore == nil || filterActive) {
			continue
		}
		scoreHdr := "N/A"
		if c.OverallScore != nil {
			scoreHdr = fmt.Sprintf("%.2f%%", *c.OverallScore)
		}
		gradeHdr := "-"
		if c.OverallGrade != nil && *c.OverallGrade != "" {
			gradeHdr = *c.OverallGrade
		}

		fmt.Printf("╭─ %s%s%s — %s%s%s (Current: %s%s%s | Grade: %s%s%s)\n",
			ColorBoldCyan, c.StudentName, ColorReset,
			ColorBoldWhite, c.CourseName, ColorReset,
			ColorBoldGreen, scoreHdr, ColorReset,
			ColorBoldYellow, gradeHdr, ColorReset,
		)
		fmt.Printf("│ %-38s │ %-24s │ %10s │ %8s │ %-14s │ %10s\n",
			"Assignment", "Category (Weight)", "Score", "Percent", "Status", "Due Date")
		fmt.Printf("├%s┼%s┼%s┼%s┼%s┼%s\n",
			strings.Repeat("─", 40), strings.Repeat("─", 26), strings.Repeat("─", 12),
			strings.Repeat("─", 10), strings.Repeat("─", 16), strings.Repeat("─", 12))

		for _, a := range c.Assignments {
			scoreDisp := "-"
			if a.Score != nil && a.PointsPossible != nil {
				scoreDisp = fmt.Sprintf("%s/%s", formatFloatTrim(*a.Score), formatFloatTrim(*a.PointsPossible))
			} else if a.Score != nil {
				scoreDisp = formatFloatTrim(*a.Score)
			} else if a.PointsPossible != nil {
				scoreDisp = fmt.Sprintf("-/%s", formatFloatTrim(*a.PointsPossible))
			}

			pctDisp := "-"
			if a.Percentage != nil {
				pctDisp = fmt.Sprintf("%.1f%%", *a.Percentage)
			}

			catDisp := a.GroupName
			if a.GroupWeightPercent > 0 {
				catDisp = fmt.Sprintf("%s (%s%%)", a.GroupName, formatFloatTrim(a.GroupWeightPercent))
			}

			dueDisp := "-"
			if a.DueAt != nil && len(*a.DueAt) >= 10 {
				dueDisp = (*a.DueAt)[:10]
			}

			statusColor := ColorYellow
			switch a.Status {
			case "Graded":
				statusColor = ColorGreen
			case "Missing":
				statusColor = ColorBoldRed
			case "Excused":
				statusColor = ColorCyan
			}

			fmt.Printf("│ %-38s │ %-24s │ %10s │ %8s │ %s%-14s%s │ %10s\n",
				truncate(a.Name, 38),
				truncate(catDisp, 24),
				scoreDisp,
				pctDisp,
				statusColor, truncate(a.Status, 14), ColorReset,
				dueDisp,
			)
		}
		fmt.Printf("╰%s\n\n", strings.Repeat("─", 115))
	}
	if cacheNotice != "" {
		fmt.Printf("%s⚡ [USING CACHED RESULTS] %s%s\n\n", ColorBoldYellow, cacheNotice, ColorReset)
	}
}

func printHelp() {
	fmt.Printf(`%sCanvas LMS Grade & Assignment Scraper%s
Extracts courses, overall scores/grades, category weights, and assignment details
from Canvas LMS into JSON/CSV files and formatted terminal tables.

%sUSAGE:%s
  ./canvas-scraper [flags]

%sGENERAL, AUTHENTICATION & OUTPUT FLAGS:%s
  --help, -h             Show this help guide and exit
  --fresh                Bypass local CACHE_TIMER and fetch live data from Canvas
  --headed               Launch visible browser window for login/MFA debugging
  --output-dir <dir>     Directory to save exported files (.env: OUTPUT_DIR=output)
  --web                  Generate a single-file HTML report (<output-dir>/canvas_grades.html) and open it
  --markdown             Generate an AI-ready Markdown report (<output-dir>/canvas_grades.md)

%sSTUDENT & COURSE FILTERS:%s
  --student <name>       Filter by student name (e.g. --student Emma or --student Tyler)
  --course <name|id>     Filter by course name or ID (e.g. --course "Algebra" or --course "Math")

%sDUE DATE & TIME-WINDOW FILTERS:%s
  --no-future            Exclude ungraded assignments with a future due date
                         (.env: HIDE_FUTURE_DUE=true)
  --future-only          Show ONLY assignments with a future due date
                         (.env: FUTURE_ONLY=true)
  --due-within <dur>     Show ONLY upcoming assignments due within <dur> from now (e.g. 7d, 14d, 48h)
                         (.env: DUE_WITHIN=7d)
  --due-past <dur>       Show ONLY assignments that were due within the past <dur> (e.g. 14d, 30d)
                         (.env: DUE_PAST=14d)
  --no-empty-due         Exclude ungraded assignments that have no due date set
                         (.env: HIDE_NO_DUE_DATE=true)

%sMISSING, STATUS & GRADE IMPACT FILTERS:%s
  --missing-days <N>     Show ONLY assignments marked Missing OR past due by >= N days with no score
                         Use 0 for anything currently past due (e.g. --missing-days 0 or --missing-days 2)
                         (.env: MISSING_AFTER_DAYS=2)
  --status <list>        Filter by comma-separated status: missing, late, graded, ungraded, excused
                         (e.g. --status missing,late) (.env: STATUS_FILTER=missing,late)
  --below <percent>      Show ONLY graded assignments scoring below <percent>%% (e.g. --below 75)
                         (.env: SCORE_BELOW=75)
  --min-weight <pct>     Show ONLY assignments in categories weighted >= <pct>%% (e.g. --min-weight 30)
                         (.env: MIN_CATEGORY_WEIGHT=30)

%sSORTING OPTIONS:%s
  --sort <field>         Sort assignments (and courses) across all outputs (.env: SORT_BY=<field>):
                         • date, date-asc       Due date (oldest to newest)
                         • date-desc            Due date (newest/upcoming to oldest)
                         • status               Priority order (Missing/Overdue -> Late -> Submitted -> Graded)
                         • score, score-asc     Score percentage (lowest to highest — spots low grades first)
                         • score-desc           Score percentage (highest to lowest)
                         • grade, grade-asc     Letter grade / percentage (A to F, highest to lowest)
                         • grade-desc           Letter grade / percentage (F to A, lowest to highest)
                         • weight, weight-desc  Category weight (highest weight %% to lowest)

%sCONFIGURATION (.env FILE OPTIONS):%s
  Settings are read from (highest priority first):
    1. Environment variables with the same names (e.g. export CANVAS_URL=...)
    2. ./.env                              (current directory)
    3. <executable dir>/.env               (same directory as canvas-scraper)
    4. ~/.config/canvas-scraper/.env       (per-user config)
  All files found are merged; a higher-priority source overrides a lower one key-by-key.

  CANVAS_URL=https://...       (Required) Base URL of your Canvas LMS instance
  CANVAS_USER_ID=...           Login username/email for Canvas
  CANVAS_PASSWORD=...          Login password for Canvas
  CANVAS_API_TOKEN=...         Optional API token (bypasses browser login if provided)
  OUTPUT_DIR=output            Directory where JSON, CSV, HTML, MD, and cache files are saved
  CACHE_TIMER=10m              Cache duration (e.g. 30s, 10m, 1h, or 0 to disable)
  SKIP_NONGRADED=true          Skip non-graded courses (homeroom, counseling, etc.)
  SKIP_UNGRADED_ITEMS=true     Exclude "Not Graded" / 0%% weight categories & assignments

%sEXAMPLES:%s
  ./canvas-scraper                                   # Standard run using .env settings & cache
  ./canvas-scraper --fresh                           # Ignore cache and pull live grades from Canvas
  ./canvas-scraper --web                             # Generate HTML dashboard and open in browser
  ./canvas-scraper --markdown                        # Generate AI-ready Markdown report (output/canvas_grades.md)
  ./canvas-scraper --no-future                       # Hide upcoming future assignments
  ./canvas-scraper --future-only                     # Show only upcoming assignments with future due dates
  ./canvas-scraper --due-within 7d                   # Show assignments due in the next 7 days
  ./canvas-scraper --missing-days 2                  # Show missing/unsubmitted items >= 2 days past due
  ./canvas-scraper --student Tyler --below 75        # Show Tyler's graded assignments below 75%%
  ./canvas-scraper --status missing,late             # Show all missing or late assignments
  ./canvas-scraper --sort date-desc                  # Sort assignments by due date (newest first)
  ./canvas-scraper --sort status                     # Sort assignments with Missing & Late at the top
  ./canvas-scraper --sort score                      # Sort courses & assignments from lowest score to highest
`,
		ColorBoldCyan, ColorReset,
		ColorBoldYellow, ColorReset,
		ColorBoldYellow, ColorReset,
		ColorBoldYellow, ColorReset,
		ColorBoldYellow, ColorReset,
		ColorBoldYellow, ColorReset,
		ColorBoldYellow, ColorReset,
		ColorBoldYellow, ColorReset,
		ColorBoldYellow, ColorReset,
	)
}

func main() {
	flag.Usage = printHelp

	headed := flag.Bool("headed", false, "Launch browser in headed mode (visible window) for MFA or debugging")
	fresh := flag.Bool("fresh", false, "Bypass local CACHE_TIMER and fetch live data from Canvas")
	outputDir := flag.String("output-dir", "", "Directory to save exported JSON/CSV/HTML/MD files (overrides OUTPUT_DIR in .env)")
	webFlag := flag.Bool("web", false, "Generate a single-file HTML report and open it in the browser")
	markdownFlag := flag.Bool("markdown", false, "Generate an AI-ready Markdown report (canvas_grades.md)")
	courseFilter := flag.String("course", "", "Filter results by specific course ID or name")
	studentFilter := flag.String("student", "", "Filter results by student name (e.g., Emma or Tyler)")

	// Assignment filter CLI flags
	noFutureFlag := flag.Bool("no-future", false, "Exclude ungraded assignments with a future due date")
	futureOnlyFlag := flag.Bool("future-only", false, "Show ONLY assignments with a future due date")
	dueWithinFlag := flag.String("due-within", "", "Show ONLY assignments due within duration from now (e.g. 7d, 14d)")
	duePastFlag := flag.String("due-past", "", "Show ONLY assignments due within the past duration (e.g. 14d, 30d)")
	noEmptyDueFlag := flag.Bool("no-empty-due", false, "Exclude ungraded assignments with no due date")
	missingDaysFlag := flag.Int("missing-days", -1, "Show ONLY assignments missing or overdue by >= N days (0 = any past due)")
	statusFlag := flag.String("status", "", "Filter assignments by status: missing, late, graded, ungraded, excused")
	belowFlag := flag.Float64("below", 0, "Show ONLY graded assignments with percentage below X%")
	minWeightFlag := flag.Float64("min-weight", 0, "Show ONLY assignments in categories weighted >= X%")
	sortFlag := flag.String("sort", "", "Sort results by: date, date-desc, status, score, score-desc, grade, grade-desc, weight")

	flag.Parse()

	cfg := loadConfigFromEnv()
	if cfg.CanvasURL == "" {
		fmt.Printf("%sError:%s Missing CANVAS_URL!\nSet CANVAS_URL (e.g. CANVAS_URL=https://your-school.instructure.com) as an environment variable or in one of these .env files:\n", ColorBoldRed, ColorReset)
		for _, p := range dotEnvSearchPaths() {
			fmt.Printf("  - %s\n", p)
		}
		os.Exit(1)
	}

	// Override .env filter defaults with any CLI flags explicitly passed
	if *outputDir != "" {
		cfg.OutputDir = expandHomePath(*outputDir)
	}
	*outputDir = cfg.OutputDir
	if *studentFilter != "" {
		cfg.StudentFilter = *studentFilter
	}
	if *courseFilter != "" {
		cfg.CourseFilter = *courseFilter
	}
	if *noFutureFlag {
		cfg.HideFutureDue = true
	}
	if *futureOnlyFlag {
		cfg.FutureOnly = true
	}
	if *noEmptyDueFlag {
		cfg.HideNoDueDate = true
	}
	if *dueWithinFlag != "" {
		if strings.EqualFold(*dueWithinFlag, "future") || strings.EqualFold(*dueWithinFlag, "all") {
			cfg.FutureOnly = true
		} else if d, err := parseFlexibleDuration(*dueWithinFlag); err == nil {
			cfg.DueWithin = d
		} else {
			fmt.Printf("%sError:%s Invalid --due-within duration %q (use e.g. 7d, 14d, 48h)\n", ColorBoldRed, *dueWithinFlag, ColorReset)
			os.Exit(1)
		}
	}
	if *duePastFlag != "" {
		if d, err := parseFlexibleDuration(*duePastFlag); err == nil {
			cfg.DuePast = d
		} else {
			fmt.Printf("%sError:%s Invalid --due-past duration %q (use e.g. 7d, 14d, 30d)\n", ColorBoldRed, *duePastFlag, ColorReset)
			os.Exit(1)
		}
	}
	if *missingDaysFlag >= 0 {
		cfg.MissingAfterDays = *missingDaysFlag
	}
	if *statusFlag != "" {
		cfg.StatusFilter = *statusFlag
	}
	if *belowFlag > 0 {
		cfg.ScoreBelow = *belowFlag
	}
	if *minWeightFlag > 0 {
		cfg.MinCategoryWeight = *minWeightFlag
	}
	if *sortFlag != "" {
		cfg.SortBy = *sortFlag
	}

	var data []CourseGradeRecord
	usedCache := false
	cacheNotice := ""
	cacheFilePath := filepath.Join(*outputDir, ".cache_raw.json")

	// C. Check local cache if CACHE_TIMER is configured and --fresh is not passed
	if *fresh {
		fmt.Printf("%s🔄 [FRESH RUN] Bypassing local cache (--fresh flag set). Fetching live data from Canvas...%s\n", ColorBoldCyan, ColorReset)
	} else if cfg.CacheTimer > 0 {
		checkPath := cacheFilePath
		usedFallback := false
		if _, err := os.Stat(checkPath); err != nil {
			checkPath = filepath.Join(*outputDir, "canvas_grades.json")
			usedFallback = true
		}
		if info, err := os.Stat(checkPath); err == nil {
			if rawBytes, err := os.ReadFile(checkPath); err == nil {
				var env CacheEnvelope
				var cached []CourseGradeRecord
				var scrapedAt time.Time

				if err := json.Unmarshal(rawBytes, &env); err == nil && len(env.Courses) > 0 && !env.ScrapedAt.IsZero() {
					cached = env.Courses
					scrapedAt = env.ScrapedAt
				} else if err := json.Unmarshal(rawBytes, &cached); err == nil && len(cached) > 0 {
					scrapedAt = info.ModTime()
				}

				if len(cached) > 0 {
					exactAge := time.Since(scrapedAt)
					age := exactAge.Round(time.Second)
					if exactAge < cfg.CacheTimer {
						remaining := (cfg.CacheTimer - exactAge).Round(time.Second)
						cacheNotice = fmt.Sprintf("Loaded from local cache (age: %v | expires in: %v | CACHE_TIMER=%v). Run with --fresh to fetch live data.",
							age, remaining, cfg.CacheTimer)
						fmt.Printf("%s⚡ [USING CACHED RESULTS] %s%s\n", ColorBoldYellow, cacheNotice, ColorReset)
						data = cached
						usedCache = true

						// Persist CacheEnvelope to .cache_raw.json if migrated from legacy array or fallback file
						if usedFallback || env.ScrapedAt.IsZero() {
							_ = os.MkdirAll(*outputDir, 0755)
							migratedEnv := CacheEnvelope{
								ScrapedAt: scrapedAt,
								Courses:   cached,
							}
							if envBytes, err := json.MarshalIndent(migratedEnv, "", "  "); err == nil {
								_ = os.WriteFile(cacheFilePath, envBytes, 0644)
							}
						}
					} else {
						fmt.Printf("%s⏰ [CACHE EXPIRED] Local cache age is %v (CACHE_TIMER=%v). Fetching fresh data from Canvas...%s\n",
							ColorBoldYellow, age, cfg.CacheTimer, ColorReset)
					}
				}
			}
		}
	}

	if !usedCache {
		var client *CanvasClient

		if cfg.APIToken != "" {
			fmt.Printf("%sFound CANVAS_API_TOKEN in environment. Using direct REST API mode.%s\n", ColorBoldGreen, ColorReset)
			client = NewCanvasClientWithToken(cfg.CanvasURL, cfg.APIToken)
		} else {
			var cookies []StoredCookie
			needLogin := false

			if !needLogin {
				if loaded, err := loadSessionCookies(SessionFile); err == nil && len(loaded) > 0 {
					fmt.Printf("%sFound existing session file: %s. Reusing session...%s\n", ColorCyan, SessionFile, ColorReset)
					testClient, err := NewCanvasClientWithCookies(cfg.CanvasURL, loaded)
					if err == nil {
						var selfUser CanvasUser
						if err := testClient.getJSON("/api/v1/users/self", &selfUser); err == nil && selfUser.ID != 0 {
							client = testClient
						} else {
							fmt.Printf("%sSaved session expired. Logging in again...%s\n", ColorYellow, ColorReset)
							needLogin = true
						}
					} else {
						needLogin = true
					}
				} else {
					needLogin = true
				}
			}

			if needLogin {
				if cfg.UserID == "" || cfg.Password == "" {
					fmt.Printf("%sError:%s Missing login credentials!\nPlease create a .env file with:\n  CANVAS_URL=https://your-school.instructure.com\n  CANVAS_USER_ID=your_id_or_email\n  CANVAS_PASSWORD=your_password\n", ColorBoldRed, ColorReset)
					os.Exit(1)
				}
				var err error
				cookies, err = authenticateWithBrowser(cfg.CanvasURL, cfg.UserID, cfg.Password, !*headed)
				if err != nil {
					fmt.Printf("%sExecution error:%s %v\n", ColorBoldRed, ColorReset, err)
					os.Exit(1)
				}
				client, err = NewCanvasClientWithCookies(cfg.CanvasURL, cookies)
				if err != nil {
					fmt.Printf("%sExecution error:%s %v\n", ColorBoldRed, ColorReset, err)
					os.Exit(1)
				}
			}
		}

		fmt.Printf("%sAuthenticated successfully. Fetching students, courses, and grades...%s\n", ColorGreen, ColorReset)
		var err error
		data, err = scrapeCanvasData(client, cfg.SkipNonGraded)
		if err != nil {
			fmt.Printf("%sExecution error:%s %v\n", ColorBoldRed, ColorReset, err)
			os.Exit(1)
		}

		// Save raw scraped data with explicit ScrapedAt timestamp to .cache_raw.json
		_ = os.MkdirAll(*outputDir, 0755)
		env := CacheEnvelope{
			ScrapedAt: time.Now(),
			Courses:   data,
		}
		if rawBytes, err := json.MarshalIndent(env, "", "  "); err == nil {
			_ = os.WriteFile(cacheFilePath, rawBytes, 0644)
		}
	} else if cfg.SkipNonGraded {
		// Apply SKIP_NONGRADED filter if loaded from an older unfiltered cache
		var graded []CourseGradeRecord
		for _, c := range data {
			if c.OverallScore != nil || (c.OverallGrade != nil && *c.OverallGrade != "") {
				graded = append(graded, c)
			}
		}
		data = graded
	}

	if cfg.SkipUngradedItems {
		data = filterUngradedItems(data)
	}

	if cfg.StudentFilter != "" {
		sf := strings.ToLower(cfg.StudentFilter)
		var filtered []CourseGradeRecord
		for _, c := range data {
			if strings.Contains(strings.ToLower(c.StudentName), sf) {
				filtered = append(filtered, c)
			}
		}
		data = filtered
	}

	if cfg.CourseFilter != "" {
		cf := strings.ToLower(cfg.CourseFilter)
		var filtered []CourseGradeRecord
		for _, c := range data {
			if strings.Contains(strings.ToLower(strconv.Itoa(c.CourseID)), cf) || strings.Contains(strings.ToLower(c.CourseName), cf) {
				filtered = append(filtered, c)
			}
		}
		data = filtered
	}

	data = applyAssignmentFilters(data, cfg)
	if cfg.SortBy != "" {
		data = applySorting(data, cfg.SortBy)
	}

	if len(data) == 0 {
		fmt.Printf("%sNo courses found matching your filters.%s\n", ColorYellow, ColorReset)
		return
	}

	if err := exportData(*outputDir, data); err != nil {
		fmt.Printf("%sExport error:%s %v\n", ColorBoldRed, ColorReset, err)
		os.Exit(1)
	}

	if *markdownFlag {
		if _, err := exportMarkdown(*outputDir, data, cacheNotice); err != nil {
			fmt.Printf("%sMarkdown export error:%s %v\n", ColorBoldRed, ColorReset, err)
			os.Exit(1)
		}
	}

	if *webFlag {
		htmlPath, err := exportHTML(*outputDir, data, cacheNotice)
		if err != nil {
			fmt.Printf("%sHTML export error:%s %v\n", ColorBoldRed, ColorReset, err)
			os.Exit(1)
		}
		if err := openInBrowser(htmlPath); err != nil {
			fmt.Printf("%sWarning: Could not automatically open browser: %v%s\n", ColorYellow, err, ColorReset)
		} else {
			fmt.Printf("%sOpened HTML report in default browser.%s\n", ColorBoldCyan, ColorReset)
		}
	}

	displayTerminalSummary(data, cacheNotice, hasActiveAssignmentFilters(cfg))
}

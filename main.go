package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
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
	DefaultCanvasURL  = "https://mtparanschool.instructure.com"
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
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	GroupWeight float64        `json:"group_weight"`
	Rules       map[string]any `json:"rules"`
}

type CanvasAssignment struct {
	ID                int               `json:"id"`
	Name              string            `json:"name"`
	AssignmentGroupID int               `json:"assignment_group_id"`
	DueAt             *string           `json:"due_at"`
	PointsPossible    *float64          `json:"points_possible"`
	HTMLURL           *string           `json:"html_url"`
	Submission        *CanvasSubmission `json:"submission"`
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

// loadDotEnv reads .env file if present and sets environment variables that aren't already set.
func loadDotEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
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
		// Strip surrounding quotes if present
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func loadCredentials() (canvasURL, userID, password, apiToken string) {
	loadDotEnv(".env")
	canvasURL = strings.TrimRight(strings.TrimSpace(os.Getenv("CANVAS_URL")), "/")
	if canvasURL == "" {
		canvasURL = DefaultCanvasURL
	}
	userID = strings.TrimSpace(os.Getenv("CANVAS_USER_ID"))
	password = strings.TrimSpace(os.Getenv("CANVAS_PASSWORD"))
	apiToken = strings.TrimSpace(os.Getenv("CANVAS_API_TOKEN"))
	return
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
		HTMLURL:            a.HTMLURL,
	}
}

func scrapeCanvasData(client *CanvasClient) ([]CourseGradeRecord, error) {
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

		var rawGroups []CanvasAssignmentGroup
		_ = client.getJSON(fmt.Sprintf("/api/v1/courses/%d/assignment_groups?per_page=100", courseID), &rawGroups)

		var rawAssignments []CanvasAssignment
		_ = client.getJSON(fmt.Sprintf("/api/v1/courses/%d/assignments?per_page=100", courseID), &rawAssignments)

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

func displayTerminalSummary(data []CourseGradeRecord) {
	fmt.Println()
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
}

func main() {
	headed := flag.Bool("headed", false, "Launch browser in headed mode (visible window) for MFA or debugging")
	fresh := flag.Bool("fresh", false, "Ignore saved session and force fresh login")
	outputDir := flag.String("output-dir", DefaultOutputDir, "Directory to save exported JSON/CSV files")
	courseFilter := flag.String("course", "", "Filter results by specific course ID or name")
	studentFilter := flag.String("student", "", "Filter results by student name (e.g., Emma or Tyler)")
	flag.Parse()

	canvasURL, userID, password, apiToken := loadCredentials()

	var client *CanvasClient

	if apiToken != "" {
		fmt.Printf("%sFound CANVAS_API_TOKEN in environment. Using direct REST API mode.%s\n", ColorBoldGreen, ColorReset)
		client = NewCanvasClientWithToken(canvasURL, apiToken)
	} else {
		var cookies []StoredCookie
		needLogin := *fresh

		if !needLogin {
			if loaded, err := loadSessionCookies(SessionFile); err == nil && len(loaded) > 0 {
				fmt.Printf("%sFound existing session file: %s. Reusing session...%s\n", ColorCyan, SessionFile, ColorReset)
				testClient, err := NewCanvasClientWithCookies(canvasURL, loaded)
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
			if userID == "" || password == "" {
				fmt.Printf("%sError:%s Missing login credentials!\nPlease create a .env file with:\n  CANVAS_USER_ID=your_id_or_email\n  CANVAS_PASSWORD=your_password\n", ColorBoldRed, ColorReset)
				os.Exit(1)
			}
			var err error
			cookies, err = authenticateWithBrowser(canvasURL, userID, password, !*headed)
			if err != nil {
				fmt.Printf("%sExecution error:%s %v\n", ColorBoldRed, ColorReset, err)
				os.Exit(1)
			}
			client, err = NewCanvasClientWithCookies(canvasURL, cookies)
			if err != nil {
				fmt.Printf("%sExecution error:%s %v\n", ColorBoldRed, ColorReset, err)
				os.Exit(1)
			}
		}
	}

	fmt.Printf("%sAuthenticated successfully. Fetching students, courses, and grades...%s\n", ColorGreen, ColorReset)
	data, err := scrapeCanvasData(client)
	if err != nil {
		fmt.Printf("%sExecution error:%s %v\n", ColorBoldRed, ColorReset, err)
		os.Exit(1)
	}

	if *studentFilter != "" {
		sf := strings.ToLower(*studentFilter)
		var filtered []CourseGradeRecord
		for _, c := range data {
			if strings.Contains(strings.ToLower(c.StudentName), sf) {
				filtered = append(filtered, c)
			}
		}
		data = filtered
	}

	if *courseFilter != "" {
		cf := strings.ToLower(*courseFilter)
		var filtered []CourseGradeRecord
		for _, c := range data {
			if strings.Contains(strings.ToLower(strconv.Itoa(c.CourseID)), cf) || strings.Contains(strings.ToLower(c.CourseName), cf) {
				filtered = append(filtered, c)
			}
		}
		data = filtered
	}

	if len(data) == 0 {
		fmt.Printf("%sNo courses found matching your filters.%s\n", ColorYellow, ColorReset)
		return
	}

	if err := exportData(*outputDir, data); err != nil {
		fmt.Printf("%sExport error:%s %v\n", ColorBoldRed, ColorReset, err)
		os.Exit(1)
	}

	displayTerminalSummary(data)
}

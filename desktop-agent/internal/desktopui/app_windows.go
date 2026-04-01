//go:build windows && !no_gl

package desktopui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"queue_up/desktop-agent/internal/appicon"
	"queue_up/desktop-agent/internal/client"
	"queue_up/desktop-agent/internal/config"
)

func Run(cfg config.Config) error {
	if strings.TrimSpace(cfg.BackendBaseURL) == "" {
		return fmt.Errorf("backend_base_url is required")
	}

	a := app.NewWithID("queueup.desktop")
	a.SetIcon(fyne.NewStaticResource("queue_up.ico", appicon.Bytes()))
	w := a.NewWindow("Queue Up Desktop")
	w.Resize(fyne.NewSize(980, 720))

	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	state := &uiState{
		cfg:        cfg,
		httpClient: httpClient,
		userID:     strings.TrimSpace(cfg.UserID),
	}

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("LeetCode username")
	submissionEntry := widget.NewEntry()
	submissionEntry.SetPlaceHolder("Required submission URL")
	statusLabel := widget.NewLabel("Sign in with your LeetCode username.")
	selectedCategoryLabel := widget.NewLabel("Selected category: none")

	queueSelect := widget.NewSelect([]string{}, func(selected string) {
		state.selectedQueueOption = selected
	})
	historyList := widget.NewMultiLineEntry()
	historyList.Wrapping = fyne.TextWrapWord
	historyList.Disable()
	leetCodeHistoryList := widget.NewList(
		func() int {
			return len(state.leetCodeHistory)
		},
		func() fyne.CanvasObject {
			title := widget.NewLabel("")
			title.Wrapping = fyne.TextWrapWord
			title.TextStyle = fyne.TextStyle{Bold: true}
			meta := widget.NewLabel("")
			meta.Wrapping = fyne.TextWrapWord
			link := widget.NewLabel("")
			link.Wrapping = fyne.TextWrapWord
			return container.NewVBox(title, meta, link, widget.NewSeparator())
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			if i < 0 || i >= len(state.leetCodeHistory) {
				return
			}
			row := state.leetCodeHistory[i]
			box := item.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			meta := box.Objects[1].(*widget.Label)
			link := box.Objects[2].(*widget.Label)
			title.SetText(row.Title)
			meta.SetText("Accepted at " + formatLeetCodeTimestamp(row.Timestamp))
			link.SetText("https://leetcode.com/problems/" + row.TitleSlug + "/")
		},
	)
	leetCodeHistoryList.HideSeparators = true
	conceptsGrid := container.NewGridWithColumns(3)

	setStatus := func(msg string) {
		statusLabel.SetText(msg)
	}

	var renderConceptOptions func()
	renderConceptOptions = func() {
		conceptsGrid.Objects = nil
		for _, concept := range state.concepts {
			c := concept
			label := c.DisplayName
			if strings.EqualFold(strings.TrimSpace(c.Code), strings.TrimSpace(state.selectedConceptCode)) {
				label = "✓ " + label
			}
			btn := widget.NewButton(label, func() {
				state.selectedConceptCode = strings.TrimSpace(c.Code)
				selectedCategoryLabel.SetText("Selected category: " + c.DisplayName)
				renderConceptOptions()
			})
			conceptsGrid.Add(btn)
		}
		conceptsGrid.Refresh()
	}

	refreshHistory := func() {
		if state.userID == "" {
			historyList.SetText("")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		out, err := client.FetchHistory(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID, 50)
		if err != nil {
			setStatus("History load failed: " + err.Error())
			return
		}
		lines := make([]string, 0, len(out.History))
		for _, h := range out.History {
			lines = append(lines, fmt.Sprintf("%s [%s] %s", h.CompletedAt, h.Difficulty, h.Title))
		}
		historyList.SetText(strings.Join(lines, "\n"))
	}

	refreshLeetCodeHistory := func() {
		username := strings.TrimSpace(state.leetcodeUsername)
		if username == "" {
			username = strings.TrimSpace(usernameEntry.Text)
		}
		if username == "" {
			dialog.ShowError(fmt.Errorf("leetcode username is required"), w)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rows, err := client.FetchRecentACSubmissions(ctx, state.httpClient, username, 20)
		if err != nil {
			setStatus("LeetCode history load failed: " + err.Error())
			return
		}
		state.leetCodeHistory = rows
		leetCodeHistoryList.Refresh()
		setStatus(fmt.Sprintf("LeetCode history loaded: %d entries", len(rows)))
	}

	applyQueue := func(recommendations []client.Recommendation, source string) {
		total := len(recommendations)
		if len(recommendations) > 3 {
			recommendations = recommendations[:3]
		}
		state.queue = recommendations
		state.queueByOption = make(map[string]client.Recommendation, len(state.queue))
		options := make([]string, 0, len(state.queue))
		for i, q := range state.queue {
			option := fmt.Sprintf("%d. %s (%s)", i+1, q.Title, q.Difficulty)
			options = append(options, option)
			state.queueByOption[option] = q
		}
		queueSelect.SetOptions(options)
		if len(options) > 0 {
			queueSelect.SetSelected(options[0])
			state.selectedQueueOption = options[0]
		} else {
			state.selectedQueueOption = ""
		}
		switch {
		case total == 3:
			setStatus(fmt.Sprintf("Queue refreshed: 3 problems (%s)", source))
		case total > 3:
			setStatus(fmt.Sprintf("Queue refreshed: showing first 3 of %d (%s)", total, source))
		default:
			setStatus(fmt.Sprintf("Queue refreshed: %d problems (%s)", total, source))
		}
	}

	refreshQueue := func() {
		if state.userID == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		out, err := client.RefreshQueue(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID)
		if err == nil && len(out.Recommendations) > 0 {
			applyQueue(out.Recommendations, "refresh endpoint")
			return
		}

		fallback, fallbackErr := client.FetchTodayRecommendations(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID)
		if fallbackErr == nil && len(fallback) > 0 {
			applyQueue(fallback, "today recommendation fallback")
			return
		}

		queueSelect.SetOptions([]string{})
		state.selectedQueueOption = ""
		if err != nil {
			setStatus("Queue refresh failed: " + err.Error())
			return
		}
		if fallbackErr != nil {
			setStatus("Queue fallback failed: " + fallbackErr.Error())
			return
		}
		setStatus("Queue refreshed: 0 problems")
	}

	loadConcepts := func(selected []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		out, err := client.FetchConcepts(ctx, state.httpClient, state.cfg.BackendBaseURL)
		if err != nil {
			setStatus("Concept load failed: " + err.Error())
			return
		}
		state.concepts = out.Concepts
		selectedByCode := make(map[string]struct{}, len(selected))
		for _, code := range selected {
			selectedByCode[strings.ToUpper(strings.TrimSpace(code))] = struct{}{}
		}
		state.selectedConceptCode = ""
		for _, c := range out.Concepts {
			if _, ok := selectedByCode[strings.ToUpper(c.Code)]; ok {
				state.selectedConceptCode = strings.TrimSpace(c.Code)
				selectedCategoryLabel.SetText("Selected category: " + c.DisplayName)
				break
			}
		}
		if state.selectedConceptCode == "" {
			selectedCategoryLabel.SetText("Selected category: none")
		}
		renderConceptOptions()
	}

	var tabs *container.AppTabs
	var studyTab *container.TabItem

	loginBtn := widget.NewButton("Verify + Continue", func() {
		username := strings.TrimSpace(usernameEntry.Text)
		if username == "" {
			dialog.ShowError(fmt.Errorf("LeetCode username is required"), w)
			return
		}
		timezone := "America/New_York"
		if loc := time.Now().Location(); loc != nil && strings.TrimSpace(loc.String()) != "" {
			timezone = loc.String()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := client.BootstrapUser(ctx, state.httpClient, state.cfg.BackendBaseURL, username, timezone)
		if err != nil {
			dialog.ShowError(err, w)
			setStatus("Login failed.")
			return
		}
		state.userID = out.Profile.UserID
		state.leetcodeUsername = strings.TrimSpace(out.Profile.LeetCodeUsername)
		if state.leetcodeUsername == "" {
			state.leetcodeUsername = username
		}
		vStatus := out.VerificationStatus
		if vStatus == "" {
			vStatus = "skipped"
		}
		setStatus(fmt.Sprintf("User ready. verification=%s", vStatus))
		loadConcepts(out.Profile.ConceptCodes)
		refreshQueue()
		refreshHistory()
		refreshLeetCodeHistory()
		if strings.TrimSpace(state.selectedConceptCode) == "" && studyTab != nil && tabs != nil {
			setStatus("Pick one study category in the Study Category tab, then save it.")
			tabs.Select(studyTab)
		}
	})

	saveConceptsBtn := widget.NewButton("Save Categories", func() {
		if state.userID == "" {
			dialog.ShowError(fmt.Errorf("login first"), w)
			return
		}
		code := strings.TrimSpace(state.selectedConceptCode)
		if code == "" {
			dialog.ShowError(fmt.Errorf("select one category"), w)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		if err := client.UpdateConcepts(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID, []string{code}); err != nil {
			dialog.ShowError(err, w)
			return
		}
		setStatus("Study category saved.")
		refreshQueue()
	})

	refreshQueueBtn := widget.NewButton("Refresh Queue", refreshQueue)
	refreshLeetCodeHistoryBtn := widget.NewButton("Refresh LeetCode History", refreshLeetCodeHistory)

	markCompleteBtn := widget.NewButton("Mark Selected Complete", func() {
		if state.userID == "" {
			dialog.ShowError(fmt.Errorf("login first"), w)
			return
		}
		selected := strings.TrimSpace(state.selectedQueueOption)
		if selected == "" {
			dialog.ShowError(fmt.Errorf("select a queue problem first"), w)
			return
		}
		rec, ok := state.queueByOption[selected]
		if !ok {
			dialog.ShowError(fmt.Errorf("selected queue item was not found"), w)
			return
		}
		submissionURL := strings.TrimSpace(submissionEntry.Text)
		if submissionURL == "" {
			dialog.ShowError(fmt.Errorf("submission URL is required"), w)
			return
		}
		parsed, parseErr := url.ParseRequestURI(submissionURL)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			dialog.ShowError(fmt.Errorf("submission URL must be a valid http(s) URL"), w)
			return
		}

		problemID := rec.ProblemID
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if problemID <= 0 {
			slug := strings.TrimSpace(rec.Slug)
			if slug == "" {
				slug = extractProblemSlugFromURL(submissionURL)
			}
			if slug == "" {
				dialog.ShowError(fmt.Errorf("unable to resolve problem id; missing title slug"), w)
				return
			}
			resolvedID, resolveErr := client.ResolveProblemFrontendID(ctx, state.httpClient, slug)
			if resolveErr != nil {
				dialog.ShowError(fmt.Errorf("resolve problem id via LeetCode failed: %w", resolveErr), w)
				return
			}
			problemID = resolvedID
		}

		if err := client.MarkCompletion(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID, problemID, submissionURL); err != nil {
			dialog.ShowError(err, w)
			return
		}
		setStatus("Problem marked complete.")
		refreshHistory()
	})

	top := container.NewVBox(
		widget.NewLabel("Login / Bootstrap"),
		container.NewBorder(nil, nil, nil, loginBtn, usernameEntry),
		statusLabel,
	)
	queue := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("Problem Queue"),
		queueSelect,
		widget.NewLabel("Submission URL (required)"),
		submissionEntry,
		container.NewHBox(markCompleteBtn, refreshQueueBtn),
	)
	history := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("Completed Problems"),
		historyList,
	)
	leetCodeHistoryHeader := container.NewVBox(
		widget.NewLabel("LeetCode Accepted Submissions"),
		widget.NewLabel("Latest accepted submissions from leetcode.com/graphql."),
		container.NewHBox(refreshLeetCodeHistoryBtn),
	)
	leetCodeHistory := container.NewBorder(
		leetCodeHistoryHeader,
		nil,
		nil,
		nil,
		leetCodeHistoryList,
	)

	dashboardContent := container.NewVBox(top, queue, history)
	studyContent := container.NewVBox(
		widget.NewLabel("Choose one study category."),
		widget.NewLabel("You can change this later from this tab."),
		selectedCategoryLabel,
		conceptsGrid,
		container.NewHBox(saveConceptsBtn),
	)

	dashboardTab := container.NewTabItem("Dashboard", container.NewPadded(dashboardContent))
	studyTab = container.NewTabItem("Study Category", container.NewPadded(studyContent))
	leetCodeHistoryTab := container.NewTabItem("LeetCode History", container.NewPadded(leetCodeHistory))
	tabs = container.NewAppTabs(dashboardTab, studyTab, leetCodeHistoryTab)
	tabs.SetTabLocation(container.TabLocationTop)

	root := container.NewBorder(
		container.NewPadded(widget.NewLabelWithStyle("Queue Up Desktop", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
		nil,
		nil,
		nil,
		tabs,
	)
	w.SetContent(root)
	w.ShowAndRun()
	return nil
}

type uiState struct {
	cfg        config.Config
	httpClient *http.Client

	userID           string
	leetcodeUsername string
	concepts         []client.Concept
	queue            []client.Recommendation
	queueByOption    map[string]client.Recommendation
	leetCodeHistory  []client.LeetCodeSubmission

	selectedConceptCode string
	selectedQueueOption string
}

func extractProblemSlugFromURL(submissionURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(submissionURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(parsed.Path, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "problems" && strings.TrimSpace(parts[i+1]) != "" {
			return strings.TrimSpace(parts[i+1])
		}
	}
	return ""
}

func formatLeetCodeTimestamp(raw string) string {
	secs, err := time.ParseDuration(strings.TrimSpace(raw) + "s")
	if err != nil {
		return raw
	}
	return time.Unix(int64(secs.Seconds()), 0).Local().Format("2006-01-02 15:04")
}

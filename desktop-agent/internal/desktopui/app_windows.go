//go:build windows && !no_gl

package desktopui

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"queue_up/desktop-agent/internal/appicon"
	"queue_up/desktop-agent/internal/client"
	"queue_up/desktop-agent/internal/config"
	"queue_up/desktop-agent/internal/enforcer"
)

const (
	prefKeyLastLeetCodeUsername = "last_leetcode_username"
	prefKeyLastUserID           = "last_user_id"
	prefKeyLastConceptName      = "last_concept_name"
	prefKeyLastConceptCode      = "last_concept_code"
)

var supportedConceptCodes = []string{
	"ARRAY",
	"TWO_POINTERS",
	"STACK",
	"BINARY_SEARCH",
	"SLIDING_WINDOW",
	"LINKED_LIST",
	"TREE",
	"TRIE",
	"BACKTRACKING",
	"HEAP",
	"GRAPH",
	"DP_1D",
	"DP_2D",
	"INTERVALS",
	"GREEDY",
	"BIT_MANIPULATION",
	"DSU",
	"QUEUE",
	"MATH_GEOMETRY",
}

var supportedConceptSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(supportedConceptCodes))
	for _, code := range supportedConceptCodes {
		m[strings.ToUpper(strings.TrimSpace(code))] = struct{}{}
	}
	return m
}()

func filterSupportedConcepts(concepts []client.Concept) []client.Concept {
	selected := make(map[string]client.Concept, len(concepts))
	for _, concept := range concepts {
		code := strings.TrimSpace(strings.ToUpper(concept.Code))
		if _, ok := supportedConceptSet[code]; !ok {
			continue
		}
		selected[code] = concept
	}
	out := make([]client.Concept, 0, len(selected))
	for _, code := range supportedConceptCodes {
		if concept, ok := selected[code]; ok {
			out = append(out, concept)
		}
	}
	return out
}

func Run(cfg config.Config, configPath string) error {
	if strings.TrimSpace(cfg.BackendBaseURL) == "" {
		return fmt.Errorf("backend_base_url is required")
	}

	release, err := acquireDashboardInstanceLock()
	if err != nil {
		return err
	}
	defer release()

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
	prefs := a.Preferences()
	rememberedUsername := strings.TrimSpace(prefs.String(prefKeyLastLeetCodeUsername))
	rememberedUserID := strings.TrimSpace(prefs.String(prefKeyLastUserID))
	rememberedConceptName := strings.TrimSpace(prefs.String(prefKeyLastConceptName))
	rememberedConceptCode := strings.TrimSpace(prefs.String(prefKeyLastConceptCode))
	if state.userID == "" && rememberedUserID != "" {
		state.userID = rememberedUserID
	}

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("LeetCode username")
	if rememberedUsername != "" {
		usernameEntry.SetText(rememberedUsername)
		state.leetcodeUsername = rememberedUsername
	}
	submissionEntry := widget.NewEntry()
	submissionEntry.SetPlaceHolder("Required submission URL")
	statusLabel := widget.NewLabel("Sign in with your LeetCode username.")
	todayWarningText := canvas.NewText("No Problems Submitted Today", color.White)
	todayWarningText.TextStyle = fyne.TextStyle{Bold: true}
	todayWarningText.Alignment = fyne.TextAlignCenter
	todayWarningText.TextSize = 12
	todayWarningBg := canvas.NewRectangle(color.NRGBA{R: 196, G: 48, B: 48, A: 255})
	todayWarningBg.CornerRadius = 10
	todayWarningPill := container.NewStack(todayWarningBg, container.NewPadded(todayWarningText))
	todayWarningPill.Hide()
	studyPlanSavedText := canvas.NewText("Study plan updated successfully", color.White)
	studyPlanSavedText.TextStyle = fyne.TextStyle{Bold: true}
	studyPlanSavedText.Alignment = fyne.TextAlignCenter
	studyPlanSavedText.TextSize = 12
	studyPlanSavedBg := canvas.NewRectangle(color.NRGBA{R: 20, G: 138, B: 78, A: 255})
	studyPlanSavedBg.CornerRadius = 10
	studyPlanSavedPill := container.NewStack(studyPlanSavedBg, container.NewPadded(studyPlanSavedText))
	studyPlanSavedPill.Hide()
	selectedCategoryLabel := widget.NewLabel("Selected category: none")
	categoryPillText := canvas.NewText("No category", color.White)
	categoryPillText.TextStyle = fyne.TextStyle{Bold: true}
	categoryPillText.Alignment = fyne.TextAlignCenter
	categoryPillText.TextSize = 12
	categoryPillBg := canvas.NewRectangle(color.NRGBA{R: 16, G: 140, B: 84, A: 255})
	categoryPillBg.CornerRadius = 10
	categoryPill := container.NewStack(categoryPillBg, container.NewPadded(categoryPillText))
	if rememberedConceptName != "" {
		categoryPillText.Text = rememberedConceptName
		categoryPillText.Refresh()
	}

	queueSelect := widget.NewSelect([]string{}, func(selected string) {
		state.selectedQueueOption = selected
	})
	completedTodayList := widget.NewList(
		func() int {
			return len(state.todayLeetCodeHistory)
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
			if i < 0 || i >= len(state.todayLeetCodeHistory) {
				return
			}
			row := state.todayLeetCodeHistory[i]
			box := item.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			meta := box.Objects[1].(*widget.Label)
			link := box.Objects[2].(*widget.Label)
			title.SetText("✓ " + row.Title)
			meta.SetText("Accepted at " + formatLeetCodeTimestamp(row.Timestamp))
			link.SetText("https://leetcode.com/problems/" + row.TitleSlug + "/")
		},
	)
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
	syncActiveUserID := func() {
		if strings.TrimSpace(configPath) == "" || strings.TrimSpace(state.userID) == "" {
			return
		}
		if err := config.UpdateUserID(configPath, state.userID); err != nil {
			log.Printf("sync user_id to config failed: %v", err)
		}
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
				prefs.SetString(prefKeyLastConceptName, c.DisplayName)
				prefs.SetString(prefKeyLastConceptCode, state.selectedConceptCode)
				categoryPillText.Text = c.DisplayName
				categoryPillText.Refresh()
				studyPlanSavedPill.Hide()
				renderConceptOptions()
			})
			conceptsGrid.Add(btn)
		}
		conceptsGrid.Refresh()
	}

	updateTodaySubmissionView := func() {
		loc := time.Now().Location()
		today := time.Now().In(loc).Format("2006-01-02")
		filtered := make([]client.LeetCodeSubmission, 0, len(state.leetCodeHistory))
		for _, row := range state.leetCodeHistory {
			t, ok := parseLeetCodeTimestamp(row.Timestamp)
			if !ok {
				continue
			}
			if t.In(loc).Format("2006-01-02") == today {
				filtered = append(filtered, row)
			}
		}
		state.todayLeetCodeHistory = filtered
		completedTodayList.Refresh()
		for i := 0; i < len(state.todayLeetCodeHistory); i++ {
			completedTodayList.SetItemHeight(i, 112)
		}
		if len(state.todayLeetCodeHistory) == 0 {
			todayWarningPill.Show()
		} else {
			todayWarningPill.Hide()
		}
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
		for i := 0; i < len(state.leetCodeHistory); i++ {
			leetCodeHistoryList.SetItemHeight(i, 112)
		}
		updateTodaySubmissionView()
		setStatus(fmt.Sprintf("LeetCode history loaded: %d entries (%d today)", len(rows), len(state.todayLeetCodeHistory)))
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
		// Clear stale queue UI immediately so category changes feel instant.
		queueSelect.SetOptions([]string{})
		state.selectedQueueOption = ""
		state.queue = nil
		state.queueByOption = map[string]client.Recommendation{}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		out, err := client.FetchProblemQueue(
			ctx,
			state.httpClient,
			state.cfg.BackendBaseURL,
			state.userID,
			state.selectedConceptCode,
		)
		if err != nil {
			setStatus("Queue refresh failed: " + err.Error())
			return
		}
		if len(out.Recommendations) == 0 {
			setStatus("Queue refreshed: 0 problems")
			return
		}
		applyQueue(out.Recommendations, out.Source)
	}

	openTodaysProblem := func() {
		if state.userID == "" {
			dialog.ShowError(fmt.Errorf("login first"), w)
			return
		}

		queueToOpen := state.queue
		if len(queueToOpen) == 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out, err := client.FetchProblemQueue(
				ctx,
				state.httpClient,
				state.cfg.BackendBaseURL,
				state.userID,
				state.selectedConceptCode,
			)
			if err != nil {
				dialog.ShowError(fmt.Errorf("queue load failed: %w", err), w)
				return
			}
			queueToOpen = out.Recommendations
			applyQueue(out.Recommendations, out.Source)
		}
		if len(queueToOpen) == 0 {
			dialog.ShowError(fmt.Errorf("problem queue is empty"), w)
			return
		}

		first := queueToOpen[0]
		targetURL := strings.TrimSpace(first.URL)
		if targetURL == "" && strings.TrimSpace(first.Slug) != "" {
			targetURL = fmt.Sprintf("https://leetcode.com/problems/%s/", strings.TrimSpace(first.Slug))
		}
		if targetURL == "" {
			dialog.ShowError(fmt.Errorf("selected queue problem has no URL"), w)
			return
		}
		if err := enforcer.OpenInDefaultBrowser(targetURL); err != nil {
			dialog.ShowError(err, w)
			return
		}
		setStatus("Opened first problem in queue.")
	}

	loadConcepts := func(selected []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		out, err := client.FetchConcepts(ctx, state.httpClient, state.cfg.BackendBaseURL)
		if err != nil {
			setStatus("Concept load failed: " + err.Error())
			return
		}
		state.concepts = filterSupportedConcepts(out.Concepts)
		selectedByCode := make(map[string]struct{}, len(selected))
		for _, code := range selected {
			selectedByCode[strings.ToUpper(strings.TrimSpace(code))] = struct{}{}
		}
		state.selectedConceptCode = ""
		for _, c := range out.Concepts {
			if _, ok := selectedByCode[strings.ToUpper(c.Code)]; ok {
				state.selectedConceptCode = strings.TrimSpace(c.Code)
				selectedCategoryLabel.SetText("Selected category: " + c.DisplayName)
				prefs.SetString(prefKeyLastConceptName, c.DisplayName)
				categoryPillText.Text = c.DisplayName
				categoryPillText.Refresh()
				break
			}
		}
		if state.selectedConceptCode == "" {
			selectedCategoryLabel.SetText("Selected category: none")
			categoryPillText.Text = "No category"
			categoryPillText.Refresh()
		}
		renderConceptOptions()
	}

	var tabs *container.AppTabs
	var dashboardTab *container.TabItem
	var studyTab *container.TabItem
	doLogin := func() {
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
		prefs.SetString(prefKeyLastLeetCodeUsername, state.leetcodeUsername)
		prefs.SetString(prefKeyLastUserID, state.userID)
		syncActiveUserID()
		vStatus := out.VerificationStatus
		if vStatus == "" {
			vStatus = "skipped"
		}
		setStatus(fmt.Sprintf("User ready. verification=%s", vStatus))
		loadConcepts(out.Profile.ConceptCodes)
		refreshQueue()
		refreshLeetCodeHistory()
		if strings.TrimSpace(state.selectedConceptCode) == "" && studyTab != nil && tabs != nil {
			setStatus("Pick one study category in the Study Category tab, then save it.")
			tabs.Select(studyTab)
		}
	}
	loginBtn := widget.NewButton("Verify + Continue", doLogin)
	usernameEntry.OnSubmitted = func(string) {
		doLogin()
	}

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
		for _, c := range state.concepts {
			if strings.EqualFold(strings.TrimSpace(c.Code), code) {
				prefs.SetString(prefKeyLastConceptName, c.DisplayName)
				categoryPillText.Text = c.DisplayName
				categoryPillText.Refresh()
				break
			}
		}
		setStatus("Study category saved.")
		studyPlanSavedPill.Show()
		refreshQueue()
		if tabs != nil && dashboardTab != nil {
			tabs.Select(dashboardTab)
		}
	})

	refreshLeetCodeHistoryBtn := widget.NewButton("Refresh LeetCode History", refreshLeetCodeHistory)
	openTodayBtn := widget.NewButton("Open Problem", openTodaysProblem)

	markCompleteBtn := widget.NewButton("Mark Selected Complete", func() {
		if state.userID == "" {
			dialog.ShowError(fmt.Errorf("login first"), w)
			return
		}
		selected := strings.TrimSpace(queueSelect.Selected)
		if selected == "" {
			selected = strings.TrimSpace(state.selectedQueueOption)
		}
		if selected == "" && len(queueSelect.Options) == 1 {
			selected = queueSelect.Options[0]
			queueSelect.SetSelected(selected)
			state.selectedQueueOption = selected
		}
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
			title := strings.TrimSpace(rec.Title)

			if dailyQueueOut, dailyErr := client.FetchDailyQueue(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID); dailyErr == nil {
				for _, item := range dailyQueueOut.Queue {
					if slug != "" && strings.EqualFold(strings.TrimSpace(item.Slug), slug) {
						problemID = item.ProblemID
						break
					}
					if title != "" && strings.EqualFold(strings.TrimSpace(item.Title), title) {
						problemID = item.ProblemID
						break
					}
				}
			}

			if problemID <= 0 {
				if todays, todayErr := client.FetchTodayRecommendations(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID); todayErr == nil {
					for _, candidate := range todays {
						if slug != "" && strings.EqualFold(strings.TrimSpace(candidate.Slug), slug) {
							problemID = candidate.ProblemID
							break
						}
						if title != "" && strings.EqualFold(strings.TrimSpace(candidate.Title), title) {
							problemID = candidate.ProblemID
							break
						}
					}
				}
			}

			if problemID <= 0 {
				dialog.ShowError(fmt.Errorf("unable to resolve backend problem id for completion"), w)
				return
			}
		}

		if err := client.MarkCompletion(ctx, state.httpClient, state.cfg.BackendBaseURL, state.userID, problemID, submissionURL); err != nil {
			dialog.ShowError(err, w)
			return
		}
		setStatus("Problem marked complete.")
		refreshLeetCodeHistory()
		refreshQueue()
	})

	top := container.NewVBox(
		container.NewBorder(nil, nil, nil, todayWarningPill, widget.NewLabel("Login / Bootstrap")),
		container.NewBorder(nil, nil, nil, loginBtn, usernameEntry),
		statusLabel,
	)
	queue := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(widget.NewLabel("Problem Queue"), categoryPill, openTodayBtn),
		queueSelect,
		widget.NewLabel("Submission URL (required)"),
		submissionEntry,
		container.NewHBox(markCompleteBtn),
	)
	history := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("Today's LeetCode Submissions"),
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

	dashboardTop := container.NewVBox(top, queue, history)
	dashboardContent := container.NewBorder(
		dashboardTop,
		nil,
		nil,
		nil,
		completedTodayList,
	)
	studyContent := container.NewVBox(
		container.NewBorder(nil, nil, nil, studyPlanSavedPill, widget.NewLabel("Choose one study category.")),
		widget.NewLabel("You can change this later from this tab."),
		selectedCategoryLabel,
		conceptsGrid,
		container.NewHBox(saveConceptsBtn),
	)

	dashboardTab = container.NewTabItem("Dashboard", container.NewPadded(dashboardContent))
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
	if state.userID != "" {
		syncActiveUserID()
		initialSelectedCodes := []string{}
		if rememberedConceptCode != "" {
			initialSelectedCodes = append(initialSelectedCodes, rememberedConceptCode)
		}
		startupUsername := strings.TrimSpace(state.leetcodeUsername)
		if startupUsername == "" {
			startupUsername = strings.TrimSpace(usernameEntry.Text)
		}
		if startupUsername != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
			lookup, lookupErr := client.LookupUserByLeetCode(ctx, state.httpClient, state.cfg.BackendBaseURL, startupUsername)
			cancel()
			if lookupErr == nil && lookup.Exists {
				state.userID = strings.TrimSpace(lookup.Profile.UserID)
				if state.userID != "" {
					prefs.SetString(prefKeyLastUserID, state.userID)
					syncActiveUserID()
				}
				if len(lookup.Profile.ConceptCodes) > 0 {
					initialSelectedCodes = lookup.Profile.ConceptCodes
				}
			}
		}
		loadConcepts(initialSelectedCodes)
		refreshQueue()
		refreshLeetCodeHistory()
	}
	w.ShowAndRun()
	return nil
}

type uiState struct {
	cfg        config.Config
	httpClient *http.Client

	userID               string
	leetcodeUsername     string
	concepts             []client.Concept
	queue                []client.Recommendation
	queueByOption        map[string]client.Recommendation
	todayLeetCodeHistory []client.LeetCodeSubmission
	leetCodeHistory      []client.LeetCodeSubmission

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
	t, ok := parseLeetCodeTimestamp(raw)
	if !ok {
		return raw
	}
	return t.Local().Format("2006-01-02 15:04")
}

func parseLeetCodeTimestamp(raw string) (time.Time, bool) {
	secs, err := time.ParseDuration(strings.TrimSpace(raw) + "s")
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(int64(secs.Seconds()), 0), true
}

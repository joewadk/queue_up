package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"queue_up/desktop-agent/internal/client"
	"queue_up/desktop-agent/internal/config"
	"queue_up/desktop-agent/internal/desktopui"
	"queue_up/desktop-agent/internal/detector"
	"queue_up/desktop-agent/internal/enforcer"
	"queue_up/desktop-agent/internal/eventlogger"
	"queue_up/desktop-agent/internal/startup"
	"queue_up/desktop-agent/internal/tray"
)

func main() {
	configPath := flag.String("config", "", "Path to desktop agent config JSON")
	trayMode := flag.Bool("tray", false, "Run with system tray UI")
	desktopMode := flag.Bool("desktop-ui", false, "Run native desktop UI")
	installStartup := flag.Bool("install-startup", false, "Register this executable in Windows Startup Apps")
	uninstallStartup := flag.Bool("uninstall-startup", false, "Remove this executable from Windows Startup Apps")
	startupStatus := flag.Bool("startup-status", false, "Print Windows Startup Apps registration status")
	startupName := flag.String("startup-name", "QueueUpDesktopAgent", "Startup entry name in HKCU Run registry key")
	flag.Parse()

	if *configPath == "" {
		*configPath = defaultConfigPath()
	}
	absConfigPath, err := filepath.Abs(*configPath)
	if err == nil {
		*configPath = absConfigPath
	}

	if *installStartup || *uninstallStartup || *startupStatus {
		if err := runStartupCommand(*installStartup, *uninstallStartup, *startupStatus, *startupName, *configPath); err != nil {
			log.Fatalf("startup command failed: %v", err)
		}
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if *desktopMode {
		if err := desktopui.Run(cfg, *configPath); err != nil {
			log.Fatalf("desktop ui failed: %v", err)
		}
		return
	}

	logger, err := eventlogger.NewJSONL(cfg.LogFilePath)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer logger.Close()

	fmt.Printf("queue-up-agent started | poll=%s cooldown=%s watched=%v\n",
		cfg.PollInterval, cfg.Cooldown, cfg.WatchedExecutables)
	if cfg.BackendBaseURL != "" && cfg.UserID != "" {
		fmt.Printf("backend recommender enabled | base=%s user_id=%s\n", cfg.BackendBaseURL, cfg.UserID)
	} else {
		fmt.Printf("backend recommender disabled | using fallback_url=%s\n", cfg.LeetCodeProblemURL)
	}

	lastTriggered := make(map[string]time.Time)
	httpClient := &http.Client{Timeout: cfg.RequestTimeout}

	if *trayMode || cfg.EnableTray {
		rt := newAgentRuntime(cfg, *configPath, logger, lastTriggered, httpClient)
		rt.startLoop()
		log.Printf("tray mode enabled")
		if cfg.OpenGUIOnStart {
			rt.OpenDashboard()
		}
		tray.Run(rt)
		rt.wait()
		return
	}

	runAgentLoop(context.Background(), cfg, logger, lastTriggered, httpClient)
}

func runOnce(cfg config.Config, logger *eventlogger.JSONL, lastTriggered map[string]time.Time, httpClient *http.Client) {
	processes, err := detector.ListRunningExecutables()
	if err != nil {
		log.Printf("list processes: %v", err)
		return
	}

	now := time.Now().UTC()
	matched := make([]string, 0, len(cfg.WatchedExecutables))
	for _, exe := range cfg.WatchedExecutables {
		if _, found := processes[exe]; !found {
			continue
		}
		matched = append(matched, exe)

		last, hasLast := lastTriggered[exe]
		if hasLast && now.Sub(last) < cfg.Cooldown {
			continue
		}

		problemURL, source, slug, shouldOpen, fetchErr := resolveProblemURL(cfg, httpClient)
		action := "OPEN_LEETCODE_PROBLEM"
		if !shouldOpen {
			action = "OPEN_LEETCODE_PROBLEM_SKIPPED"
		}
		event := eventlogger.EnforcementEvent{
			TimestampUTC:           now,
			Executable:             exe,
			Action:                 action,
			ProblemURL:             problemURL,
			RecommendationSource:   source,
			RecommendedProblemSlug: slug,
			DryRun:                 cfg.DryRun,
		}
		if fetchErr != nil {
			event.Error = fetchErr.Error()
		}

		if shouldOpen {
			if cfg.DryRun {
				log.Printf("detected %s (dry-run): would open %s source=%s", exe, problemURL, source)
			} else {
				if err := enforcer.OpenInDefaultBrowser(problemURL); err != nil {
					event.Action = "OPEN_LEETCODE_PROBLEM_FAILED"
					if event.Error == "" {
						event.Error = err.Error()
					} else {
						event.Error = event.Error + "; " + err.Error()
					}
					log.Printf("open browser for %s: %v", exe, err)
				} else {
					log.Printf("detected %s: opened %s source=%s", exe, problemURL, source)
				}
			}
		} else {
			reason := "skipped opening problem"
			if fetchErr != nil {
				reason = fetchErr.Error()
			}
			log.Printf("detected %s: %s", exe, reason)
		}

		if err := logger.Write(event); err != nil {
			log.Printf("write event log: %v", err)
		}

		lastTriggered[exe] = now
	}

	if cfg.LogPolls {
		if len(matched) == 0 {
			log.Printf("poll tick: scanned running processes, no watched executables found")
		} else {
			log.Printf("poll tick: watched executables currently running: %v", matched)
		}
	}
}

var errProblemsSubmittedToday = errors.New("user already submitted a problem today")

func resolveProblemURL(cfg config.Config, httpClient *http.Client) (problemURL, source, slug string, shouldOpen bool, fetchErr error) {
	fallback := cfg.LeetCodeProblemURL
	if strings.TrimSpace(cfg.BackendBaseURL) == "" || strings.TrimSpace(cfg.UserID) == "" {
		return fallback, "fallback_static", "", true, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
	defer cancel()

	daily, dailyErr := client.FetchDailyQueue(ctx, httpClient, cfg.BackendBaseURL, cfg.UserID)
	if dailyErr == nil && daily.CompletedCount > 0 {
		return fallback, "backend_daily_queue_submitted", "", false, errProblemsSubmittedToday
	}

	problemQueue, queueErr := client.FetchProblemQueue(ctx, httpClient, cfg.BackendBaseURL, cfg.UserID, "")
	if queueErr != nil {
		if dailyErr != nil {
			return fallback, "fallback_static", "", true, fmt.Errorf("queue load failed: %v; daily queue load failed: %w", queueErr, dailyErr)
		}
		return fallback, "fallback_static", "", true, fmt.Errorf("queue load failed: %w", queueErr)
	}
	if len(problemQueue.Recommendations) == 0 {
		return fallback, "fallback_static", "", true, errors.New("problem queue is empty")
	}

	first := problemQueue.Recommendations[0]
	problemURL, slug = buildProblemLink(first.URL, first.Slug, fallback)
	source = "problem_queue:" + problemQueue.Source
	return problemURL, source, slug, true, nil
}

func buildProblemLink(urlCandidate, slugCandidate, fallback string) (string, string) {
	urlCandidate = strings.TrimSpace(urlCandidate)
	slugCandidate = strings.TrimSpace(slugCandidate)
	if urlCandidate == "" && slugCandidate != "" {
		urlCandidate = fmt.Sprintf("https://leetcode.com/problems/%s/", slugCandidate)
	}
	if urlCandidate == "" {
		urlCandidate = fallback
	}
	return urlCandidate, slugCandidate
}

type agentRuntime struct {
	cfg           config.Config
	logger        *eventlogger.JSONL
	lastTriggered map[string]time.Time
	httpClient    *http.Client
	configPath    string

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newAgentRuntime(cfg config.Config, configPath string, logger *eventlogger.JSONL, lastTriggered map[string]time.Time, httpClient *http.Client) *agentRuntime {
	ctx, cancel := context.WithCancel(context.Background())
	return &agentRuntime{
		cfg:           cfg,
		logger:        logger,
		lastTriggered: lastTriggered,
		httpClient:    httpClient,
		configPath:    configPath,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (a *agentRuntime) startLoop() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runLoop()
	}()
}

func (a *agentRuntime) wait() {
	a.wg.Wait()
}

func (a *agentRuntime) Stop() {
	a.cancel()
}

func (a *agentRuntime) OpenToday() {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg := a.currentConfig()
	problemURL, source, slug, shouldOpen, fetchErr := resolveProblemURL(cfg, a.httpClient)
	action := "MANUAL_OPEN_TODAY"
	if !shouldOpen {
		action = "MANUAL_OPEN_TODAY_SKIPPED"
	}
	event := eventlogger.EnforcementEvent{
		TimestampUTC:           time.Now().UTC(),
		Executable:             "manual_tray",
		Action:                 action,
		ProblemURL:             problemURL,
		RecommendationSource:   source,
		RecommendedProblemSlug: slug,
		DryRun:                 cfg.DryRun,
	}
	if fetchErr != nil {
		event.Error = fetchErr.Error()
	}

	if shouldOpen {
		if cfg.DryRun {
			log.Printf("tray open today (dry-run): would open %s source=%s", problemURL, source)
		} else {
			if err := enforcer.OpenInDefaultBrowser(problemURL); err != nil {
				event.Action = "MANUAL_OPEN_TODAY_FAILED"
				if event.Error == "" {
					event.Error = err.Error()
				} else {
					event.Error = event.Error + "; " + err.Error()
				}
				log.Printf("tray open today failed: %v", err)
			} else {
				log.Printf("tray open today: %s source=%s", problemURL, source)
			}
		}
	} else {
		reason := "skipped opening problem"
		if fetchErr != nil {
			reason = fetchErr.Error()
		}
		log.Printf("tray open today: %s", reason)
	}

	if err := a.logger.Write(event); err != nil {
		log.Printf("write event log: %v", err)
	}
}

func (a *agentRuntime) MarkDone() {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg := a.currentConfig()
	event := eventlogger.EnforcementEvent{
		TimestampUTC: time.Now().UTC(),
		Executable:   "manual_tray",
		Action:       "MANUAL_MARK_DONE",
		ProblemURL:   cfg.LeetCodeProblemURL,
		DryRun:       cfg.DryRun,
	}

	if strings.TrimSpace(cfg.BackendBaseURL) == "" || strings.TrimSpace(cfg.UserID) == "" {
		event.Action = "MANUAL_MARK_DONE_FAILED"
		event.Error = "backend_base_url and user_id are required"
	} else if !cfg.DryRun {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
		defer cancel()
		problemID, err := client.MarkFirstIncompleteToday(ctx, a.httpClient, cfg.BackendBaseURL, cfg.UserID)
		if err != nil {
			event.Action = "MANUAL_MARK_DONE_FAILED"
			event.Error = err.Error()
		} else {
			event.RecommendedProblemSlug = fmt.Sprintf("problem_id:%d", problemID)
			event.RecommendationSource = "backend_daily_queue"
		}
	}

	if err := a.logger.Write(event); err != nil {
		log.Printf("write event log: %v", err)
	}
	if event.Error != "" {
		log.Printf("tray mark done error: %s", event.Error)
		return
	}
	if cfg.DryRun {
		log.Printf("tray mark done (dry-run)")
		return
	}
	log.Printf("tray marked completion successfully")
}

func (a *agentRuntime) OpenDashboard() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := launchDesktopUI(a.configPath); err != nil {
		log.Printf("launch desktop ui: %v", err)
		return
	}
	log.Printf("desktop ui launched")
}

func launchDesktopUI(configPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	args := []string{"-desktop-ui"}
	if strings.TrimSpace(configPath) != "" {
		args = append(args, "-config", configPath)
	}
	cmd := exec.Command(exePath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start desktop ui process: %w", err)
	}
	return nil
}

func (a *agentRuntime) currentConfig() config.Config {
	if strings.TrimSpace(a.configPath) == "" {
		return a.cfg
	}
	latest, err := config.Load(a.configPath)
	if err != nil {
		log.Printf("reload config failed; using in-memory config: %v", err)
		return a.cfg
	}
	return latest
}

func (a *agentRuntime) runLoop() {
	current := a.currentConfig()
	if current.PollInterval <= 0 {
		current.PollInterval = a.cfg.PollInterval
	}
	runOnce(current, a.logger, a.lastTriggered, a.httpClient)

	ticker := time.NewTicker(current.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			log.Printf("agent loop stopped")
			return
		case <-ticker.C:
			next := a.currentConfig()
			if next.PollInterval <= 0 {
				next.PollInterval = current.PollInterval
			}
			if next.PollInterval != current.PollInterval {
				ticker.Stop()
				ticker = time.NewTicker(next.PollInterval)
			}
			current = next
			runOnce(current, a.logger, a.lastTriggered, a.httpClient)
		}
	}
}

func runAgentLoop(ctx context.Context, cfg config.Config, logger *eventlogger.JSONL, lastTriggered map[string]time.Time, httpClient *http.Client) {
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	runOnce(cfg, logger, lastTriggered, httpClient)

	for {
		select {
		case <-ctx.Done():
			log.Printf("agent loop stopped")
			return
		case <-ticker.C:
			runOnce(cfg, logger, lastTriggered, httpClient)
		}
	}
}

func init() {
	// Keep Windows console window alive on fatal startup errors when double-clicked.
	if len(os.Args) == 1 {
		_ = os.Setenv("QUEUE_UP_INTERACTIVE", "1")
	}
}

func defaultConfigPath() string {
	if env := strings.TrimSpace(os.Getenv("QUEUE_UP_CONFIG")); env != "" {
		return env
	}

	wd, err := os.Getwd()
	if err == nil {
		wdConfig := filepath.Join(wd, "config", "config.json")
		if _, statErr := os.Stat(wdConfig); statErr == nil {
			return wdConfig
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return "config/config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config", "config.json")
}

func runStartupCommand(install, uninstall, status bool, startupName, configPath string) error {
	selected := 0
	if install {
		selected++
	}
	if uninstall {
		selected++
	}
	if status {
		selected++
	}
	if selected != 1 {
		return fmt.Errorf("choose exactly one of -install-startup, -uninstall-startup, or -startup-status")
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("startup registration is only supported on Windows")
	}

	switch {
	case install:
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable path: %w", err)
		}
		absConfigPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("resolve config path: %w", err)
		}
		if err := startup.Install(startupName, exePath, absConfigPath); err != nil {
			return err
		}
		fmt.Printf("startup enabled: %s\n", startupName)
		return nil
	case uninstall:
		if err := startup.Uninstall(startupName); err != nil {
			return err
		}
		fmt.Printf("startup disabled: %s\n", startupName)
		return nil
	default:
		enabled, value, err := startup.Status(startupName)
		if err != nil {
			return err
		}
		if !enabled {
			fmt.Printf("startup status: disabled (%s)\n", startupName)
			return nil
		}
		fmt.Printf("startup status: enabled (%s)\ncommand: %s\n", startupName, value)
		return nil
	}
}

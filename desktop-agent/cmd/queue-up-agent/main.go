package main

import (
	"context"
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
		if err := desktopui.Run(cfg); err != nil {
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

		problemURL, source, slug, fetchErr := resolveProblemURL(cfg, httpClient)
		event := eventlogger.EnforcementEvent{
			TimestampUTC:           now,
			Executable:             exe,
			Action:                 "OPEN_LEETCODE_PROBLEM",
			ProblemURL:             problemURL,
			RecommendationSource:   source,
			RecommendedProblemSlug: slug,
			DryRun:                 cfg.DryRun,
		}
		if fetchErr != nil {
			event.Error = fetchErr.Error()
		}

		if !cfg.DryRun {
			if err := enforcer.OpenInDefaultBrowser(problemURL); err != nil {
				event.Action = "OPEN_LEETCODE_PROBLEM_FAILED"
				if event.Error == "" {
					event.Error = err.Error()
				} else {
					event.Error = event.Error + "; " + err.Error()
				}
				log.Printf("open browser for %s: %v", exe, err)
			}
		}

		if err := logger.Write(event); err != nil {
			log.Printf("write event log: %v", err)
		}

		lastTriggered[exe] = now
		if cfg.DryRun {
			log.Printf("detected %s (dry-run): would open %s source=%s", exe, problemURL, source)
		} else {
			log.Printf("detected %s: opened %s source=%s", exe, problemURL, source)
		}
	}

	if cfg.LogPolls {
		if len(matched) == 0 {
			log.Printf("poll tick: scanned running processes, no watched executables found")
		} else {
			log.Printf("poll tick: watched executables currently running: %v", matched)
		}
	}
}

func resolveProblemURL(cfg config.Config, httpClient *http.Client) (problemURL, source, slug string, fetchErr error) {
	fallback := cfg.LeetCodeProblemURL
	if strings.TrimSpace(cfg.BackendBaseURL) == "" || strings.TrimSpace(cfg.UserID) == "" {
		return fallback, "fallback_static", "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
	defer cancel()

	rec, err := client.FetchTodayRecommendation(ctx, httpClient, cfg.BackendBaseURL, cfg.UserID)
	if err != nil {
		return fallback, "fallback_static", "", fmt.Errorf("backend recommendation failed: %w", err)
	}
	return rec.URL, "backend_today_recommendation", rec.Slug, nil
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
		runAgentLoop(a.ctx, a.cfg, a.logger, a.lastTriggered, a.httpClient)
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

	problemURL, source, slug, fetchErr := resolveProblemURL(a.cfg, a.httpClient)
	event := eventlogger.EnforcementEvent{
		TimestampUTC:           time.Now().UTC(),
		Executable:             "manual_tray",
		Action:                 "MANUAL_OPEN_TODAY",
		ProblemURL:             problemURL,
		RecommendationSource:   source,
		RecommendedProblemSlug: slug,
		DryRun:                 a.cfg.DryRun,
	}
	if fetchErr != nil {
		event.Error = fetchErr.Error()
	}
	if !a.cfg.DryRun {
		if err := enforcer.OpenInDefaultBrowser(problemURL); err != nil {
			event.Action = "MANUAL_OPEN_TODAY_FAILED"
			if event.Error == "" {
				event.Error = err.Error()
			} else {
				event.Error = event.Error + "; " + err.Error()
			}
		}
	}
	if err := a.logger.Write(event); err != nil {
		log.Printf("write event log: %v", err)
	}
	if event.Error != "" {
		log.Printf("tray open today error: %s", event.Error)
		return
	}
	log.Printf("tray open today: %s source=%s", problemURL, source)
}

func (a *agentRuntime) MarkDone() {
	a.mu.Lock()
	defer a.mu.Unlock()

	event := eventlogger.EnforcementEvent{
		TimestampUTC: time.Now().UTC(),
		Executable:   "manual_tray",
		Action:       "MANUAL_MARK_DONE",
		ProblemURL:   a.cfg.LeetCodeProblemURL,
		DryRun:       a.cfg.DryRun,
	}

	if strings.TrimSpace(a.cfg.BackendBaseURL) == "" || strings.TrimSpace(a.cfg.UserID) == "" {
		event.Action = "MANUAL_MARK_DONE_FAILED"
		event.Error = "backend_base_url and user_id are required"
	} else if !a.cfg.DryRun {
		ctx, cancel := context.WithTimeout(context.Background(), a.cfg.RequestTimeout)
		defer cancel()
		problemID, err := client.MarkFirstIncompleteToday(ctx, a.httpClient, a.cfg.BackendBaseURL, a.cfg.UserID)
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
	if a.cfg.DryRun {
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

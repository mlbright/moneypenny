package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

const configPath = "/etc/auto-shutdown.conf"

type config struct {
	AfterHour     int
	AfterMinute   int
	IdleMinutes   int
	CheckInterval time.Duration
	DryRun        bool
}

// ── configuration ───────────────────────────────────────────────────

func loadConfig() config {
	cfg, err := ini.LooseLoad(configPath)
	if err != nil {
		log.Printf("Could not read %s, using defaults: %v", configPath, err)
		cfg = ini.Empty()
	}

	section := cfg.Section("shutdown")

	afterTime := section.Key("after_time").MustString("22:00")
	parts := strings.SplitN(afterTime, ":", 2)
	hour, _ := strconv.Atoi(parts[0])
	minute := 0
	if len(parts) == 2 {
		minute, _ = strconv.Atoi(parts[1])
	}

	idleMin := section.Key("idle_minutes").MustInt(30)
	checkSec := section.Key("check_interval_seconds").MustInt(60)
	dryRun := section.Key("dry_run").MustBool(false)

	return config{
		AfterHour:     hour,
		AfterMinute:   minute,
		IdleMinutes:   idleMin,
		CheckInterval: time.Duration(checkSec) * time.Second,
		DryRun:        dryRun,
	}
}

// ── idle detection ──────────────────────────────────────────────────

func xprintidleMs() (int, bool) {
	out, err := exec.Command("xprintidle").Output()
	if err != nil {
		return 0, false
	}
	ms, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false
	}
	return ms, true
}

func loginctlIdleSeconds() (int, bool) {
	out, err := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if err != nil {
		return 0, false
	}

	lines := strings.TrimSpace(string(out))
	if lines == "" {
		return 0, false
	}

	minIdle := -1
	for _, line := range strings.Split(lines, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		sessionID := fields[0]

		props, err := exec.Command(
			"loginctl", "show-session", sessionID,
			"--property=IdleHint,IdleSinceHint",
		).Output()
		if err != nil {
			continue
		}

		info := make(map[string]string)
		for _, pl := range strings.Split(strings.TrimSpace(string(props)), "\n") {
			k, v, ok := strings.Cut(pl, "=")
			if ok {
				info[k] = v
			}
		}

		if info["IdleHint"] == "no" {
			return 0, true
		}

		if info["IdleHint"] == "yes" {
			sinceUs, err := strconv.ParseInt(info["IdleSinceHint"], 10, 64)
			if err != nil || sinceUs <= 0 {
				continue
			}
			nowUs := time.Now().UnixMicro()
			idleS := int((nowUs - sinceUs) / 1_000_000)
			if minIdle < 0 || idleS < minIdle {
				minIdle = idleS
			}
		}
	}

	if minIdle < 0 {
		return 0, false
	}
	return minIdle, true
}

func inputDevIdleSeconds() (int, bool) {
	info, err := os.Stat("/dev/input/mice")
	if err != nil {
		return 0, false
	}
	atime := atime(info)
	idle := int(time.Since(atime).Seconds())
	return idle, true
}

func getIdleSeconds() int {
	var candidates []int

	if ms, ok := xprintidleMs(); ok {
		candidates = append(candidates, ms/1000)
	}
	if s, ok := loginctlIdleSeconds(); ok {
		candidates = append(candidates, s)
	}
	if s, ok := inputDevIdleSeconds(); ok {
		candidates = append(candidates, s)
	}

	if len(candidates) == 0 {
		log.Println("WARNING: could not determine idle time from any source; assuming active")
		return 0
	}

	min := candidates[0]
	for _, c := range candidates[1:] {
		if c < min {
			min = c
		}
	}
	return min
}

// ── logged-in user activity guard ────────────────────────────────────

// hasLoggedInUserActivity returns true if any user is logged in and shows
// signs of activity.  This acts as a hard veto: we NEVER shut down while
// a user is actively present, regardless of what idle-time heuristics say.
func hasLoggedInUserActivity() bool {
	// 1. Fast check: are any users logged in at all?
	whoOut, whoErr := exec.Command("who").Output()
	whoLines := strings.TrimSpace(string(whoOut))
	if whoErr != nil || whoLines == "" {
		return false // no logged-in users detected
	}

	// 2. Users are logged in.  Ask loginctl whether any session is active
	//    (IdleHint=no).  If loginctl is unavailable, err on the side of
	//    caution and assume the user is active.
	sessOut, sessErr := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if sessErr != nil {
		log.Println("loginctl unavailable but users are logged in; assuming active")
		return true
	}

	for _, line := range strings.Split(strings.TrimSpace(string(sessOut)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		sessionID := fields[0]

		props, err := exec.Command(
			"loginctl", "show-session", sessionID,
			"--property=IdleHint",
		).Output()
		if err != nil {
			continue
		}

		for _, pl := range strings.Split(strings.TrimSpace(string(props)), "\n") {
			k, v, ok := strings.Cut(pl, "=")
			if ok && k == "IdleHint" && v == "no" {
				return true
			}
		}
	}

	return false
}

// ── time gate ───────────────────────────────────────────────────────

func isPastShutdownTime(afterHour, afterMinute int) bool {
	now := time.Now()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(),
		afterHour, afterMinute, 0, 0, now.Location())
	return !now.Before(cutoff)
}

// ── shutdown ────────────────────────────────────────────────────────

func doShutdown(dryRun bool) {
	if dryRun {
		log.Println("DRY-RUN: would execute 'shutdown -h now'")
		return
	}
	log.Println("Initiating system shutdown")
	if err := exec.Command("shutdown", "-h", "now").Run(); err != nil {
		log.Printf("ERROR: shutdown command failed: %v", err)
	}
}

// ── main loop ───────────────────────────────────────────────────────

func main() {
	cfg := loadConfig()

	log.Printf("auto-shutdown started: shutdown after %02d:%02d, "+
		"idle threshold %d min, check every %s, dry_run=%v",
		cfg.AfterHour, cfg.AfterMinute,
		cfg.IdleMinutes, cfg.CheckInterval, cfg.DryRun)

	idleThreshold := cfg.IdleMinutes * 60

	for {
		if !isPastShutdownTime(cfg.AfterHour, cfg.AfterMinute) {
			time.Sleep(cfg.CheckInterval)
			continue
		}

		if hasLoggedInUserActivity() {
			log.Println("Active user session detected; shutdown vetoed")
			time.Sleep(cfg.CheckInterval)
			continue
		}

		idle := getIdleSeconds()
		log.Printf("In shutdown window. Idle: %d s (threshold %d s)", idle, idleThreshold)

		if idle >= idleThreshold {
			doShutdown(cfg.DryRun)
			if cfg.DryRun {
				time.Sleep(cfg.CheckInterval)
				continue
			}
			fmt.Println("Shutdown issued, exiting.")
			os.Exit(0)
		}

		time.Sleep(cfg.CheckInterval)
	}
}

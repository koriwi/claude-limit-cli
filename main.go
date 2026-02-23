package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	baseURL   = "https://claude.ai/api/organizations"
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// ANSI codes
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiCyan   = "\033[36m"
)

// Nerd font icons (Material Design via nerd-fonts)
const (
	iconClaude   = "󰧱" // nf-md-robot
	iconTimer    = "󰔟" // nf-md-timer_sand
	iconCalendar = "󰸗" // nf-md-calendar_week
	iconRefresh  = "󰑓" // nf-md-restore
)

type LimitUsage struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    *string `json:"resets_at"`
}

type UsageResponse struct {
	FiveHour LimitUsage `json:"five_hour"`
	SevenDay LimitUsage `json:"seven_day"`
}

type Organization struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func fetch(url, sessionKey string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-client-platform", "web_claude_ai")
	req.Header.Set("anthropic-client-version", "1.0.0")
	req.Header.Set("user-agent", userAgent)
	req.Header.Set("origin", "https://claude.ai")
	req.Header.Set("referer", "https://claude.ai/settings/usage")
	req.Header.Set("Cookie", "sessionKey="+sessionKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case 200:
		if strings.HasPrefix(strings.TrimSpace(string(body)), "<") {
			return nil, fmt.Errorf("received HTML instead of JSON — possibly blocked by Cloudflare")
		}
		return body, nil
	case 401:
		return nil, fmt.Errorf("unauthorized — session key is invalid or expired")
	case 403:
		return nil, fmt.Errorf("forbidden — missing permissions or Cloudflare block")
	case 429:
		return nil, fmt.Errorf("rate limited — try again later")
	default:
		return nil, fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
	}
}

func fetchOrg(sessionKey string) (uuid, name string, err error) {
	body, err := fetch(baseURL, sessionKey)
	if err != nil {
		return "", "", err
	}
	var orgs []Organization
	if err := json.Unmarshal(body, &orgs); err != nil {
		return "", "", fmt.Errorf("failed to parse organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", "", fmt.Errorf("no organizations found")
	}
	return orgs[0].UUID, orgs[0].Name, nil
}

func fetchUsage(sessionKey, orgID string) (*UsageResponse, error) {
	url := fmt.Sprintf("%s/%s/usage", baseURL, orgID)
	body, err := fetch(url, sessionKey)
	if err != nil {
		return nil, err
	}
	var usage UsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("failed to parse usage response: %w", err)
	}
	return &usage, nil
}

func progressBar(util float64, width int) string {
	n := min(max(int(util/100.0*float64(width)), 0), width)
	return strings.Repeat("█", n) + strings.Repeat("░", width-n)
}

func utilColor(util float64) string {
	if util >= 85 {
		return ansiRed
	} else if util >= 60 {
		return ansiYellow
	}
	return ansiGreen
}

func formatTimeLeft(resetsAt *string) string {
	if resetsAt == nil {
		return "—"
	}
	var t time.Time
	var err error
	// Try fractional seconds first (API returns e.g. "2025-12-01T15:00:00.000Z")
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		t, err = time.Parse(layout, *resetsAt)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "unknown"
	}

	dur := time.Until(t)
	if dur <= 0 {
		return "resetting…"
	}

	days := int(dur.Hours()) / 24
	hours := int(dur.Hours()) % 24
	mins := int(dur.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func printRow(icon, label string, u LimitUsage) {
	col := utilColor(u.Utilization)
	bar := progressBar(u.Utilization, 20)
	tl := formatTimeLeft(u.ResetsAt)

	fmt.Printf("  %s  %-10s %s%s%s  %s%5.1f%%%s   %s%s %s%s\n",
		icon, label,
		col, bar, ansiReset,
		ansiBold, u.Utilization, ansiReset,
		ansiDim, iconRefresh, tl, ansiReset,
	)
}

func loadConfig() (sessionKey, orgID string) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	f, err := os.Open(filepath.Join(dir, "claude-usage", "config"))
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "session_key":
			sessionKey = strings.TrimSpace(v)
		case "org_id":
			orgID = strings.TrimSpace(v)
		}
	}
	return
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, ansiBold+ansiRed+"Error:"+ansiReset+" "+format+"\n", args...)
	os.Exit(1)
}

func printCompact(usage *UsageResponse) {
	h := usage.FiveHour
	d := usage.SevenDay
	hCol := utilColor(h.Utilization)
	dCol := utilColor(d.Utilization)
	fmt.Printf("%s%s%s %.1f%% %s   %s%s%s %.1f%% %s\n",
		hCol, iconTimer, ansiReset, h.Utilization, formatTimeLeft(h.ResetsAt),
		dCol, iconCalendar, ansiReset, d.Utilization, formatTimeLeft(d.ResetsAt),
	)
}

func main() {
	var flagKey, flagOrg string
	var flagCompact bool
	flag.StringVar(&flagKey, "session-key", "", "Claude session key (sk-ant-…)")
	flag.StringVar(&flagOrg, "org-id", "", "Organization UUID (auto-fetched if not set)")
	flag.BoolVar(&flagCompact, "compact", false, "One-line output")
	flag.Parse()

	cfgKey, cfgOrg := loadConfig()

	sessionKey := firstNonEmpty(flagKey, os.Getenv("CLAUDE_SESSION_KEY"), cfgKey)
	orgID := firstNonEmpty(flagOrg, os.Getenv("CLAUDE_ORG_ID"), cfgOrg)

	if sessionKey == "" {
		dir, _ := os.UserConfigDir()
		cfgPath := filepath.Join(dir, "claude-usage", "config")
		fmt.Fprintf(os.Stderr, ansiBold+ansiRed+"Error:"+ansiReset+" session key required\n\n")
		fmt.Fprintf(os.Stderr, "Provide it via:\n")
		fmt.Fprintf(os.Stderr, "  --session-key <key>       CLI flag\n")
		fmt.Fprintf(os.Stderr, "  CLAUDE_SESSION_KEY=<key>  environment variable\n")
		fmt.Fprintf(os.Stderr, "  %-26s  config file\n\n", cfgPath)
		fmt.Fprintf(os.Stderr, "To find your session key:\n")
		fmt.Fprintf(os.Stderr, "  1. Open claude.ai/settings/usage in your browser\n")
		fmt.Fprintf(os.Stderr, "  2. DevTools (F12) → Application → Cookies → claude.ai\n")
		fmt.Fprintf(os.Stderr, "  3. Copy the value of the 'sessionKey' cookie\n")
		os.Exit(1)
	}

	orgName := ""
	if orgID == "" {
		var err error
		orgID, orgName, err = fetchOrg(sessionKey)
		if err != nil {
			fatalf("%v", err)
		}
	}

	usage, err := fetchUsage(sessionKey, orgID)
	if err != nil {
		fatalf("%v", err)
	}

	if flagCompact {
		printCompact(usage)
		return
	}

	title := "Claude Pro Usage"
	if orgName != "" {
		title = fmt.Sprintf("Claude Pro — %s", orgName)
	}

	fmt.Printf("\n  %s%s%s %s%s\n\n", ansiBold, ansiCyan, iconClaude, title, ansiReset)
	printRow(iconTimer, "5-Hour", usage.FiveHour)
	fmt.Println()
	printRow(iconCalendar, "7-Day", usage.SevenDay)
	fmt.Println()
}

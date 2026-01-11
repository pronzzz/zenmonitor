package monitor

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pronzzz/zenmonitor/internal/config"
)

// CheckResult represents the outcome of a single check
type CheckResult struct {
	MonitorName string
	Timestamp   time.Time
	Status      bool // true = UP, false = DOWN
	Latency     time.Duration
	Error       string
}

// Store interface to decouple persistence
type Store interface {
	LogCheck(result CheckResult) error
}

// Notifier interface (optional for now, or direct call)
type Notifier interface {
	Notify(monitorName string, isUp bool, oldState bool)
}

type Engine struct {
	Cfg      *config.Config
	Store    Store
	Notifier Notifier
	// State tracking for alerting (simple map)
	lastState map[string]bool 
	mu        sync.RWMutex
	stopCh    chan struct{}
}

func NewEngine(cfg *config.Config, store Store, notifier Notifier) *Engine {
	return &Engine{
		Cfg:       cfg,
		Store:     store,
		Notifier:  notifier,
		lastState: make(map[string]bool),
		stopCh:    make(chan struct{}),
	}
}

func (e *Engine) Start() {
	for _, m := range e.Cfg.Monitors {
		go e.runMonitor(m)
	}
}

func (e *Engine) Stop() {
	close(e.stopCh)
}

func (e *Engine) runMonitor(m config.MonitorConfig) {
	// Determine interval
	interval := config.ParseDuration(e.Cfg.Global.CheckInterval)
	if m.Interval != "" {
		interval = config.ParseDuration(m.Interval)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check immediately
	e.performCheck(m)

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.performCheck(m)
		}
	}
}

func (e *Engine) performCheck(m config.MonitorConfig) {
	start := time.Now()
	var err error
	var success bool

	// Perform the check based on type
	switch m.Type {
	case "http", "https":
		success, err = checkHTTP(m)
	case "tcp":
		success, err = checkTCP(m)
	case "icmp":
		success, err = checkICMP(m) // "ping"
	default:
		// Fallback or duplicate http logic
		if m.URL != "" {
			success, err = checkHTTP(m)
		} else {
			err = fmt.Errorf("unknown monitor type")
		}
	}

	latency := time.Since(start)

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	result := CheckResult{
		MonitorName: m.Name,
		Timestamp:   start,
		Status:      success,
		Latency:     latency,
		Error:       errMsg,
	}

	// Persist
	if e.Store != nil {
		// Log error but don't stop
		_ = e.Store.LogCheck(result)
	}

	// Alerting / State Update
	e.mu.Lock()
	wasUp, exists := e.lastState[m.Name]
	e.lastState[m.Name] = success
	e.mu.Unlock()

	// If state changed, or it's the first run (maybe don't alert on first run? 
	// PRD: "Trigger alert on UP -> DOWN transition". 
	// So we need to know previous state. If new, assume it was UP or ignore?
	// Let's assume on first run, we just set state.
	if exists && wasUp != success {
		if e.Notifier != nil {
			e.Notifier.Notify(m.Name, success, wasUp)
		}
	}
}

// --- Check Implementations ---

func checkHTTP(m config.MonitorConfig) (bool, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(m.URL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != m.ExpectStatus {
		return false, fmt.Errorf("status code %d, expected %d", resp.StatusCode, m.ExpectStatus)
	}
	return true, nil
}

func checkTCP(m config.MonitorConfig) (bool, error) {
	target := fmt.Sprintf("%s:%d", m.Host, m.Port)
	conn, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

func checkICMP(m config.MonitorConfig) (bool, error) {
	// ICMP usually requires root or specialized libraries (go-ping).
	// Since we want to keep deps low/simple, we might try a simple net.Dial("ip4:icmp") 
	// but that needs root. 
	// Or execute "ping" command?
	// PRD says "ICMP (Ping)".
	// standard lib does not easily support ICMP without privileges. 
	// "github.com/prometheus-community/pro-bing" is common. 
	// For "Zen" minimal: let's try a TCP handshake to port 80? No, that's TCP.
	// Let's implement a shell-out to `ping` as a fallback, or just skip proper ICMP for now 
	// and note it.
	// Actually, let's use a "fake" ping via UDP dial? No.
	// Let's treat ICMP as "not fully implemented" or use `go-ping` if I can add the dep.
	// Since I can't run `go get`, I'll write the code assuming `exec.Command("ping")`.
	// It's safer for "no-root" containers often.
	
	// Simplified shell ping
	// ping -c 1 -W 1 host (linux)
	return false, fmt.Errorf("ICMP not yet implemented (requires decision on root vs shell)")
}

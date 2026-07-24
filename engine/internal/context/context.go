package context

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultSystemID is used when YAML does not provide system-id.
	DefaultSystemID = "ZDHZDH"

	maxDailySequence uint64 = 999999999999
)

// dailySequenceGenerator allocates one monotonically increasing sequence per
// transaction date. A root context owns one generator and all parallel clones
// share it, which guarantees uniqueness for one Engine execution.
type dailySequenceGenerator struct {
	mu       sync.Mutex
	counters map[string]uint64
}

func newDailySequenceGenerator() *dailySequenceGenerator {
	return &dailySequenceGenerator{counters: make(map[string]uint64)}
}

func (g *dailySequenceGenerator) next(now time.Time) (string, error) {
	tradeDate := now.Format("060102")

	g.mu.Lock()
	defer g.mu.Unlock()

	current := g.counters[tradeDate]
	if current >= maxDailySequence {
		return "", fmt.Errorf("daily seq_no sequence exhausted for %s", tradeDate)
	}
	current++
	g.counters[tradeDate] = current
	return fmt.Sprintf("%s%012d", tradeDate, current), nil
}

// ServiceDef defines a named service with address and optional DB.
type ServiceDef struct {
	Address  string  `yaml:"address"` // "ip:port"
	DB       *DBConf `yaml:"db,omitempty"`
	TCPPort  int     `yaml:"tcp-port,omitempty"`
	HTTPPort int     `yaml:"http-port,omitempty"`
}

// DBConf defines a database connection.
type DBConf struct {
	Address  string `yaml:"address"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Type     string `yaml:"type"`
}

// TestContext holds the global state shared across all steps in a test run.
type TestContext struct {
	mu        sync.RWMutex
	Variables map[string]string

	// Services (from YAML config) — service name → definition
	Services map[string]ServiceDef

	TestBase    string
	Flags       string
	EnvName     string
	SystemID    string
	ParentGUID  string
	Verbose     bool
	DryRun      bool
	Concurrency int  // max concurrent goroutines (0/1 = serial)
	RunAll      bool // skip all flags filtering, run every case

	sequenceGenerator *dailySequenceGenerator
}

// New creates a fresh TestContext.
func New() *TestContext {
	return &TestContext{
		Variables:         make(map[string]string),
		Services:          make(map[string]ServiceDef),
		SystemID:          DefaultSystemID,
		sequenceGenerator: newDailySequenceGenerator(),
	}
}

// GetServiceAddr returns the address for a named service.
// Returns ("", false) if the service is not found.
func (c *TestContext) GetServiceAddr(name string) (string, bool) {
	svc, ok := c.Services[name]
	if !ok {
		return "", false
	}
	return svc.Address, true
}

// GetService returns the full service definition.
func (c *TestContext) GetService(name string) (ServiceDef, bool) {
	svc, ok := c.Services[name]
	return svc, ok
}

// GetServiceDB returns the DB config for a named service, if it has one.
func (c *TestContext) GetServiceDB(name string) (*DBConf, bool) {
	svc, ok := c.Services[name]
	if !ok || svc.DB == nil {
		return nil, false
	}
	return svc.DB, true
}

// GetServiceAddrForStep returns the address for a step.
// It checks step attrs for "server" (service name) first, then falls back
// to legacy "serverIP:serverPort" variables.
func (c *TestContext) GetServiceAddrForStep(serverName string) (string, bool) {
	if serverName != "" {
		// Try named service
		if addr, ok := c.GetServiceAddr(serverName); ok {
			return addr, true
		}
		// Try as literal "ip:port"
		if strings.Contains(serverName, ":") {
			return serverName, true
		}
	}

	// Fallback to legacy serverIP/serverPort
	ip, ok1 := c.Get("serverIP")
	port, ok2 := c.Get("serverPort")
	if ok1 && ok2 {
		return ip + ":" + port, true
	}
	return "", false
}

// Get retrieves a variable value, returns ("", false) if not found.
func (c *TestContext) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.Variables[key]
	return v, ok
}

// Set stores a variable.
func (c *TestContext) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables[key] = value
}

// GetOrDefault retrieves a variable value or returns the default.
func (c *TestContext) GetOrDefault(key, def string) string {
	if v, ok := c.Get(key); ok {
		return v
	}
	return def
}

// Delete removes a variable.
func (c *TestContext) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Variables, key)
}

// Clone creates a deep copy of TestContext for parallel case execution.
// Variables and Services maps are fully copied to ensure isolation.
func (c *TestContext) Clone() *TestContext {
	sequenceGenerator := c.getSequenceGenerator()

	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := &TestContext{
		Variables:   make(map[string]string, len(c.Variables)),
		Services:    make(map[string]ServiceDef, len(c.Services)),
		TestBase:    c.TestBase,
		Flags:       c.Flags,
		EnvName:     c.EnvName,
		SystemID:    c.SystemID,
		ParentGUID:  c.ParentGUID,
		Verbose:     c.Verbose,
		DryRun:      c.DryRun,
		Concurrency: c.Concurrency,
		RunAll:      c.RunAll,

		// All case clones in one execution must allocate from the same counter.
		sequenceGenerator: sequenceGenerator,
	}

	for k, v := range c.Variables {
		clone.Variables[k] = v
	}
	for k, v := range c.Services {
		clone.Services[k] = v
	}

	return clone
}

func (c *TestContext) getSequenceGenerator() *dailySequenceGenerator {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sequenceGenerator == nil {
		c.sequenceGenerator = newDailySequenceGenerator()
	}
	return c.sequenceGenerator
}

// CleanupTemporary removes temporary variables after a test case completes.
func (c *TestContext) CleanupTemporary() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.Variables {
		// Clean up testv_* prefixed variables
		if len(k) >= 6 && k[:6] == "testv_" {
			delete(c.Variables, k)
		}
	}
	// Clean up specific session variables
	delete(c.Variables, "setup_1")
	delete(c.Variables, "teardown_1")
	delete(c.Variables, "server_index")
	delete(c.Variables, "resultVariable")
}

// GenerateSystemVars creates the per-step system variables like seq_no.
// serverName is the service name (from step's "server" field) or "" for legacy.
func (c *TestContext) GenerateSystemVars(serverName string) error {
	if err := c.generateTemporalSystemVars(time.Now()); err != nil {
		return err
	}

	// serverIP / serverPort — from service name or first service
	if serverName != "" {
		if svc, ok := c.Services[serverName]; ok {
			ip, port := splitHostPort(svc.Address)
			c.Set("serverIP", ip)
			c.Set("serverPort", port)
		}
	} else if len(c.Services) > 0 {
		if svc, ok := c.defaultService(); ok {
			ip, port := splitHostPort(svc.Address)
			c.Set("serverIP", ip)
			c.Set("serverPort", port)
		}
	}
	return nil
}

// GenerateSystemVarsLegacy is kept for backward compat with tests.
// It uses the first service instead of legacy Servers list.
func (c *TestContext) GenerateSystemVarsLegacy(serverIndex int) error {
	if err := c.generateTemporalSystemVars(time.Now()); err != nil {
		return err
	}

	// Use first service if available
	if svc, ok := c.defaultService(); ok {
		ip, port := splitHostPort(svc.Address)
		c.Set("serverIP", ip)
		c.Set("serverPort", port)
	}
	return nil
}

func (c *TestContext) defaultService() (ServiceDef, bool) {
	if len(c.Services) == 0 {
		return ServiceDef{}, false
	}
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return c.Services[names[0]], true
}

// generateTemporalSystemVars creates a 24-character transaction sequence:
// 6-letter system ID + YYMMDD + 12-digit daily increasing sequence.
func (c *TestContext) generateTemporalSystemVars(now time.Time) error {
	systemID := c.SystemID
	if systemID == "" {
		systemID = DefaultSystemID
	}
	if err := ValidateSystemID(systemID); err != nil {
		return err
	}

	dateAndSequence, err := c.getSequenceGenerator().next(now)
	if err != nil {
		return err
	}
	dateStr6 := now.Format("060102")
	timeStr6 := now.Format("06010215")
	seqNo := systemID + dateAndSequence

	c.Set("date_str_6", dateStr6)
	c.Set("time_str_6", timeStr6)
	c.Set("seq_no", seqNo)
	// Keep the legacy variable as an alias so existing templates do not break.
	c.Set("seq_no_pay", seqNo)
	c.Set("time_no", timeStr6)
	return nil
}

// ValidateSystemID checks the six-letter identifier used as the seq_no prefix.
func ValidateSystemID(systemID string) error {
	if len(systemID) != 6 {
		return fmt.Errorf("system-id must contain exactly 6 letters, got %q", systemID)
	}
	for _, ch := range systemID {
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
			return fmt.Errorf("system-id must contain only ASCII letters, got %q", systemID)
		}
	}
	return nil
}

// GenerateRandomVars creates per-case random variables.
func (c *TestContext) GenerateRandomVars() {
	c.Set("random_8", fmt.Sprintf("%08d", rand.Intn(100000000)))
}

// splitHostPort splits "ip:port" into (ip, port).
func splitHostPort(addr string) (string, string) {
	colonIdx := strings.LastIndex(addr, ":")
	if colonIdx < 0 {
		return addr, ""
	}
	return addr[:colonIdx], addr[colonIdx+1:]
}

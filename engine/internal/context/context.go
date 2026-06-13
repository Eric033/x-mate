package context

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// ServiceDef defines a named service with address and optional DB.
type ServiceDef struct {
	Address  string  `yaml:"address"`             // "ip:port"
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

	// Services (new, from YAML config) — service name → definition
	Services map[string]ServiceDef

	// Legacy fields (backward compat)
	Servers    []ServerEntry
	DBPools    []DBConfig
	DamperTCP  string // tcpDamServerIP:tcpDamServerPort
	DamperHTTP string // httpDamServerIP:httpDamServerPort

	TestBase   string
	Flags      string
	EnvName    string
	ParentGUID string
	Verbose    bool
	DryRun     bool
}

// ServerEntry represents a target server address.
type ServerEntry struct {
	IP   string
	Port string
}

// DBConfig represents a database connection configuration.
type DBConfig struct {
	IP     string
	Port   string
	Name   string
	User   string
	Passwd string
	Type   string
}

// New creates a fresh TestContext.
func New() *TestContext {
	return &TestContext{
		Variables: make(map[string]string),
		Services:  make(map[string]ServiceDef),
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
func (c *TestContext) GenerateSystemVars(serverName string) {
	now := time.Now()

	// date_str_6: YMMdd format (year last digit + month + day)
	dateStr6 := fmt.Sprintf("%d%02d%02d", now.Year()%10, now.Month(), now.Day())
	c.Set("date_str_6", dateStr6)

	// time_str_6: YMMddHH format
	timeStr6 := fmt.Sprintf("%d%02d%02d%02d", now.Year()%10, now.Month(), now.Day(), now.Hour())
	c.Set("time_str_6", timeStr6)

	// seq_no: date_str_6 + timestamp last 9 digits + "00"
	nano := now.UnixNano()
	tsStr := fmt.Sprintf("%d", nano)
	if len(tsStr) >= 9 {
		tsStr = tsStr[len(tsStr)-9:]
	}
	seqNo := dateStr6 + tsStr + "00"
	c.Set("seq_no", seqNo)

	// seq_no_pay: last 5 of date_str_6 + timestamp last 9 + "00"
	if len(dateStr6) >= 5 {
		c.Set("seq_no_pay", dateStr6[len(dateStr6)-5:]+tsStr+"00")
	}

	// time_no
	c.Set("time_no", timeStr6)

	// serverIP / serverPort — from service name or legacy
	if serverName != "" {
		if svc, ok := c.Services[serverName]; ok {
			ip, port := splitHostPort(svc.Address)
			c.Set("serverIP", ip)
			c.Set("serverPort", port)
		}
	} else if len(c.Servers) > 0 {
		// Legacy: use first server
		c.Set("serverIP", c.Servers[0].IP)
		c.Set("serverPort", c.Servers[0].Port)
	}
}

// GenerateSystemVarsLegacy is kept for backward compat with tests.
func (c *TestContext) GenerateSystemVarsLegacy(serverIndex int) {
	now := time.Now()

	dateStr6 := fmt.Sprintf("%d%02d%02d", now.Year()%10, now.Month(), now.Day())
	c.Set("date_str_6", dateStr6)

	timeStr6 := fmt.Sprintf("%d%02d%02d%02d", now.Year()%10, now.Month(), now.Day(), now.Hour())
	c.Set("time_str_6", timeStr6)

	nano := now.UnixNano()
	tsStr := fmt.Sprintf("%d", nano)
	if len(tsStr) >= 9 {
		tsStr = tsStr[len(tsStr)-9:]
	}
	seqNo := dateStr6 + tsStr + "00"
	c.Set("seq_no", seqNo)

	if len(dateStr6) >= 5 {
		c.Set("seq_no_pay", dateStr6[len(dateStr6)-5:]+tsStr+"00")
	}

	c.Set("time_no", timeStr6)

	if serverIndex > 0 && serverIndex <= len(c.Servers) {
		c.Set("serverIP", c.Servers[serverIndex-1].IP)
		c.Set("serverPort", c.Servers[serverIndex-1].Port)
	}
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
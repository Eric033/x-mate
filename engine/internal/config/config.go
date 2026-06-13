package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// AppConfig is the top-level YAML config structure.
type AppConfig struct {
	Engine   EngineConfig               `yaml:"engine"`
	Services map[string]context.ServiceDef  `yaml:"services"`
}

// EngineConfig holds engine-level settings.
type EngineConfig struct {
	TestBase   string `yaml:"test-base"`
	Flags      string `yaml:"flags"`
	Verbose    bool   `yaml:"verbose"`
	DryRun     bool   `yaml:"dry-run"`
	EnvName    string `yaml:"env-name"`
	ParentGUID string `yaml:"parent-guid"`
}

// Config holds all resolved configuration (YAML + CLI overrides).
type Config struct {
	TestBase      string
	Server        string  // kept for backward compat CLI
	Flags         string
	Verbose       bool
	DryRun        bool
	DBInfo        string  // kept for backward compat CLI
	DamperServer  string  // kept for backward compat CLI
	EnvName       string
	ParentGUID    string

	// New fields
	ConfigPath    string
	ActiveProfile string

	// Resolved services (populated from YAML)
	Services map[string]context.ServiceDef
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Flags:      "core",
		EnvName:    "UNDEFINED",
		ParentGUID: "92508788-4c1c-11e9-808b-005056a01111",
		ActiveProfile: "default",
	}
}

// LoadYAML loads and merges YAML configuration files.
// It looks for:
//   1. application.yaml (base)
//   2. application-{profile}.yaml (profile-specific, optional)
func (c *Config) LoadYAML() error {
	basePath := c.ConfigPath
	if basePath == "" {
		basePath = "application.yaml"
	}

	// Try to find the config file
	baseData, err := os.ReadFile(basePath)
	if err != nil {
		// If --config was explicitly specified, error out
		if c.ConfigPath != "" {
			return fmt.Errorf("config file %s: %w", c.ConfigPath, err)
		}
		// Otherwise try default locations
		for _, p := range []string{
			"engine.yaml",
			"config/application.yaml",
			filepath.Join(os.Getenv("HOME"), ".config", "engine", "application.yaml"),
		} {
			baseData, err = os.ReadFile(p)
			if err == nil {
				basePath = p
				break
			}
		}
		if baseData == nil {
			// No config file found; only error if no CLI overrides provided
			return nil
		}
	}

	// Parse base YAML
	appCfg, err := parseMultiDocYAML(baseData)
	if err != nil {
		return fmt.Errorf("parse %s: %w", basePath, err)
	}

	// Look for profile-specific file
	profilePath := strings.TrimSuffix(basePath, ".yaml") + "-" + c.ActiveProfile + ".yaml"
	if profileData, err := os.ReadFile(profilePath); err == nil {
		profileCfg, err := parseMultiDocYAML(profileData)
		if err != nil {
			return fmt.Errorf("parse %s: %w", profilePath, err)
		}
		appCfg = mergeAppConfig(appCfg, profileCfg)
	}

	// Apply to Config
	c.applyAppConfig(appCfg)
	return nil
}

// parseMultiDocYAML handles YAML with "---" document separators (spring profiles style).
func parseMultiDocYAML(data []byte) (*AppConfig, error) {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	var result AppConfig

	for {
		var doc AppConfig
		err := decoder.Decode(&doc)
		if err != nil {
			break
		}
		merged := mergeAppConfig(&result, &doc)
		result = *merged
	}

	return &result, nil
}

// mergeAppConfig merges src into dst (src fields override dst).
func mergeAppConfig(dst, src *AppConfig) *AppConfig {
	if src == nil {
		return dst
	}

	// Merge engine fields
	if src.Engine.TestBase != "" {
		dst.Engine.TestBase = src.Engine.TestBase
	}
	if src.Engine.Flags != "" {
		dst.Engine.Flags = src.Engine.Flags
	}
	if src.Engine.Verbose {
		dst.Engine.Verbose = src.Engine.Verbose
	}
	if src.Engine.DryRun {
		dst.Engine.DryRun = src.Engine.DryRun
	}
	if src.Engine.EnvName != "" {
		dst.Engine.EnvName = src.Engine.EnvName
	}
	if src.Engine.ParentGUID != "" {
		dst.Engine.ParentGUID = src.Engine.ParentGUID
	}

	// Merge services (deep merge by name)
	if dst.Services == nil {
		dst.Services = make(map[string]context.ServiceDef)
	}
	for name, svc := range src.Services {
		existing, exists := dst.Services[name]
		if !exists {
			dst.Services[name] = svc
			continue
		}
		// Deep merge service
		if svc.Address != "" {
			existing.Address = svc.Address
		}
		if svc.TCPPort != 0 {
			existing.TCPPort = svc.TCPPort
		}
		if svc.HTTPPort != 0 {
			existing.HTTPPort = svc.HTTPPort
		}
		if svc.DB != nil {
			if existing.DB == nil {
				existing.DB = &context.DBConf{}
			}
			if svc.DB.Address != "" {
				existing.DB.Address = svc.DB.Address
			}
			if svc.DB.Database != "" {
				existing.DB.Database = svc.DB.Database
			}
			if svc.DB.Username != "" {
				existing.DB.Username = svc.DB.Username
			}
			if svc.DB.Password != "" {
				existing.DB.Password = svc.DB.Password
			}
			if svc.DB.Type != "" {
				existing.DB.Type = svc.DB.Type
			}
		}
		dst.Services[name] = existing
	}

	return dst
}

// applyAppConfig copies AppConfig fields into the flat Config struct.
func (c *Config) applyAppConfig(appCfg *AppConfig) {
	if appCfg == nil {
		return
	}

	// Engine fields (only if not already set by CLI)
	if c.TestBase == "" {
		c.TestBase = appCfg.Engine.TestBase
	}
	if c.Flags == "core" && appCfg.Engine.Flags != "" {
		c.Flags = appCfg.Engine.Flags
	}
	if !c.Verbose {
		c.Verbose = appCfg.Engine.Verbose
	}
	if !c.DryRun {
		c.DryRun = appCfg.Engine.DryRun
	}
	if c.EnvName == "UNDEFINED" && appCfg.Engine.EnvName != "" {
		c.EnvName = appCfg.Engine.EnvName
	}
	if c.ParentGUID == "92508788-4c1c-11e9-808b-005056a01111" && appCfg.Engine.ParentGUID != "" {
		c.ParentGUID = appCfg.Engine.ParentGUID
	}

	// Services
	c.Services = appCfg.Services
}

// InitContext populates a TestContext from Config.
func (c *Config) InitContext(ctx *context.TestContext) {
	ctx.TestBase = c.TestBase
	ctx.Flags = c.Flags
	ctx.Verbose = c.Verbose
	ctx.DryRun = c.DryRun
	ctx.EnvName = c.EnvName
	ctx.ParentGUID = c.ParentGUID

	// Store services
	ctx.Services = c.Services

	// Expand services into variables: {ServiceName}.address, {ServiceName}.ip, {ServiceName}.port
	for name, svc := range c.Services {
		ip, port := splitHostPort(svc.Address)
		ctx.Set(name+".address", svc.Address)
		ctx.Set(name+".ip", ip)
		ctx.Set(name+".port", port)
		if svc.DB != nil {
			dbIP, dbPort := splitHostPort(svc.DB.Address)
			ctx.Set(name+".db.address", svc.DB.Address)
			ctx.Set(name+".db.ip", dbIP)
			ctx.Set(name+".db.port", dbPort)
			ctx.Set(name+".db.database", svc.DB.Database)
			ctx.Set(name+".db.username", svc.DB.Username)
			ctx.Set(name+".db.password", svc.DB.Password)
		}
		if svc.TCPPort > 0 {
			ctx.Set(name+".tcp-port", fmt.Sprintf("%d", svc.TCPPort))
		}
		if svc.HTTPPort > 0 {
			ctx.Set(name+".http-port", fmt.Sprintf("%d", svc.HTTPPort))
		}
	}

	// Backward compat: populate old-style vars if single service exists
	if len(c.Services) > 0 {
		// Pick first service as "default" for backward compat
		for _, svc := range c.Services {
			ip, port := splitHostPort(svc.Address)
			ctx.Set("serverIP", ip)
			ctx.Set("serverPort", port)
			if svc.DB != nil {
				dbIP, dbPort := splitHostPort(svc.DB.Address)
				ctx.Set("DB_IP", dbIP)
				ctx.Set("DB_port", dbPort)
				ctx.Set("DB_name", svc.DB.Database)
				ctx.Set("DB_user", svc.DB.Username)
				ctx.Set("DB_passwd", svc.DB.Password)
				ctx.Set("DB_type", "UNDEFINED")
			}
			break // only first
		}
	}

	// Backward compat: old-style parsed servers/db pools (from CLI fallback)
	if c.Server != "" {
		ctx.Servers = ParseServer(c.Server)
		for i, s := range ctx.Servers {
			ctx.Set(fmt.Sprintf("server_%d", i+1), s.IP)
			ctx.Set(fmt.Sprintf("port_%d", i+1), s.Port)
		}
	}

	// Damper (backward compat)
	if c.DamperServer != "" {
		tcpAddr, httpAddr := ParseDamper(c.DamperServer)
		ctx.DamperTCP = tcpAddr
		ctx.DamperHTTP = httpAddr
		damParts := strings.Split(c.DamperServer, ":")
		if len(damParts) >= 4 {
			ctx.Set("tcpDamServerIP", damParts[0])
			ctx.Set("tcpDamServerPort", damParts[2])
			ctx.Set("httpDamServerIP", damParts[0])
			ctx.Set("httpDamServerPort", damParts[3])
		}
	}

	// DB pools (backward compat)
	if c.DBInfo != "" {
		ctx.DBPools = ParseDBInfo(c.DBInfo)
		for i, db := range ctx.DBPools {
			ctx.Set(fmt.Sprintf("dbips_%d", i+1), db.IP)
			ctx.Set(fmt.Sprintf("dbports_%d", i+1), db.Port)
			ctx.Set(fmt.Sprintf("dbnames_%d", i+1), db.Name)
		}
		// Set DB_* shorthand from first DB pool
		if len(ctx.DBPools) > 0 {
			db0 := ctx.DBPools[0]
			ctx.Set("DB_IP", db0.IP)
			ctx.Set("DB_port", db0.Port)
			ctx.Set("DB_name", db0.Name)
			ctx.Set("DB_user", db0.User)
			ctx.Set("DB_passwd", db0.Passwd)
			ctx.Set("DB_type", db0.Type)
		}
	}

	// Raw config vars
	ctx.Set("testBase", c.TestBase)
	ctx.Set("flags", c.Flags)
	ctx.Set("G_server", c.Server)
	ctx.Set("G_DamplerServer", c.DamperServer)
	ctx.Set("parent_guid", c.ParentGUID)
	ctx.Set("DB_info", c.DBInfo)
	ctx.Set("envName", c.EnvName)
}

// splitHostPort splits "ip:port" into (ip, port).
func splitHostPort(addr string) (string, string) {
	colonIdx := strings.LastIndex(addr, ":")
	if colonIdx < 0 {
		return addr, ""
	}
	return addr[:colonIdx], addr[colonIdx+1:]
}

// ParseServer parses the --server flag into server entries.
// Format: "ip1:port1" or "ip1:port1@ip2:port2"
func ParseServer(raw string) []context.ServerEntry {
	var servers []context.ServerEntry
	parts := strings.Split(raw, "@")
	for _, p := range parts {
		colonIdx := strings.LastIndex(p, ":")
		if colonIdx < 0 {
			servers = append(servers, context.ServerEntry{IP: p, Port: ""})
			continue
		}
		servers = append(servers, context.ServerEntry{
			IP:   p[:colonIdx],
			Port: p[colonIdx+1:],
		})
	}
	return servers
}

// ParseDamper parses the --damper-server flag.
// Format: "ip:port:tcpPort:httpPort"
func ParseDamper(raw string) (tcpAddr, httpAddr string) {
	parts := strings.Split(raw, ":")
	if len(parts) >= 4 {
		tcpAddr = parts[0] + ":" + parts[2]
		httpAddr = parts[0] + ":" + parts[3]
	} else if len(parts) >= 2 {
		tcpAddr = parts[0] + ":" + parts[1]
		httpAddr = parts[0] + ":" + parts[1]
	}
	return
}

// ParseDBInfo parses --db-info into individual fields.
// Format: "ip:port:dbname:user:passwd" or multi-db "ip1:port1:db1:user1:pwd1@ip2:port2:db2:user2:pwd2"
func ParseDBInfo(raw string) []context.DBConfig {
	var dbs []context.DBConfig
	if raw == "" || raw == "UNDEFINED" {
		return dbs
	}

	// Split multi-db by @
	groups := strings.Split(raw, "@")
	for _, g := range groups {
		parts := strings.Split(g, ":")
		db := context.DBConfig{
			User:    "readonly",
			Passwd:  "readonly",
			Type:    "UNDEFINED",
		}
		if len(parts) >= 1 {
			db.IP = parts[0]
		}
		if len(parts) >= 2 {
			db.Port = parts[1]
		}
		if len(parts) >= 3 {
			db.Name = parts[2]
		}
		if len(parts) >= 4 {
			db.User = parts[3]
		}
		if len(parts) >= 5 {
			db.Passwd = parts[4]
		}
		dbs = append(dbs, db)
	}
	return dbs
}
package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// forbiddenEngineKeys lists YAML keys that belong to CLI flags and must NOT
// appear in environment YAML files.
var forbiddenEngineKeys = []string{
	"flags", "concurrency", "dry-run", "verbose",
	"test-base", "parent-guid", "engine",
}

// EnvironmentConfig is the top-level YAML config structure.
// It describes only the target environment — no execution parameters.
type EnvironmentConfig struct {
	SystemID    string                        `yaml:"system-id"`
	Environment EnvironmentMeta               `yaml:"environment"`
	Services    map[string]context.ServiceDef `yaml:"services"`
}

// EnvironmentMeta holds metadata about the environment.
type EnvironmentMeta struct {
	Name string `yaml:"name"`
}

// Config holds all resolved configuration (YAML + CLI overrides).
type Config struct {
	TestBase   string
	Flags      string
	Verbose    bool
	DryRun     bool
	ParentGUID string
	EnvName    string // from YAML environment.name
	SystemID   string // from YAML system-id

	ConfigPath  string
	Concurrency int // max concurrent goroutines (0/1 = serial)

	// Resolved services (populated from YAML)
	Services map[string]context.ServiceDef
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Flags:      "core",
		SystemID:   context.DefaultSystemID,
		ParentGUID: "92508788-4c1c-11e9-808b-005056a01111",
	}
}

// LoadYAML loads a single YAML environment file from ConfigPath.
// If ConfigPath is empty, it searches default locations.
func (c *Config) LoadYAML() error {
	path := c.ConfigPath
	if path == "" {
		return fmt.Errorf("config: no config path provided")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}

	// Strict validation: refuse YAML that contains forbidden engine keys
	if err := validateNoForbiddenKeys(data); err != nil {
		return fmt.Errorf("validate %s: %w", path, err)
	}

	// Parse as single document (no multi-doc support without profiles)
	var envCfg EnvironmentConfig
	if err := yaml.Unmarshal(data, &envCfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	c.applyEnvironmentConfig(&envCfg)
	if err := context.ValidateSystemID(c.SystemID); err != nil {
		return fmt.Errorf("invalid system-id: %w", err)
	}
	return nil
}

// validateNoForbiddenKeys checks that the YAML content does not contain
// top-level keys that belong in CLI flags rather than environment config.
func validateNoForbiddenKeys(data []byte) error {
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key := range raw {
		for _, forbidden := range forbiddenEngineKeys {
			if key == forbidden {
				return fmt.Errorf(
					"key %q is not allowed in environment YAML; "+
						"use CLI flags instead", key)
			}
		}
	}
	return nil
}

// applyEnvironmentConfig copies EnvironmentConfig fields into the flat Config struct.
func (c *Config) applyEnvironmentConfig(envCfg *EnvironmentConfig) {
	if envCfg == nil {
		return
	}

	if envCfg.SystemID != "" {
		c.SystemID = envCfg.SystemID
	}

	// Environment metadata
	if envCfg.Environment.Name != "" {
		c.EnvName = envCfg.Environment.Name
	}

	// Services
	c.Services = envCfg.Services
}

// InitContext populates a TestContext from Config.
func (c *Config) InitContext(ctx *context.TestContext) {
	ctx.TestBase = c.TestBase
	ctx.Flags = c.Flags
	ctx.Verbose = c.Verbose
	ctx.DryRun = c.DryRun
	ctx.EnvName = c.EnvName
	if c.SystemID != "" {
		ctx.SystemID = c.SystemID
	}
	ctx.ParentGUID = c.ParentGUID
	ctx.Concurrency = c.Concurrency

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
		serviceNames := make([]string, 0, len(c.Services))
		for name := range c.Services {
			serviceNames = append(serviceNames, name)
		}
		sort.Strings(serviceNames)
		defaultService := c.Services[serviceNames[0]]

		ip, port := splitHostPort(defaultService.Address)
		ctx.Set("serverIP", ip)
		ctx.Set("serverPort", port)
		if defaultService.DB != nil {
			dbIP, dbPort := splitHostPort(defaultService.DB.Address)
			ctx.Set("DB_IP", dbIP)
			ctx.Set("DB_port", dbPort)
			ctx.Set("DB_name", defaultService.DB.Database)
			ctx.Set("DB_user", defaultService.DB.Username)
			ctx.Set("DB_passwd", defaultService.DB.Password)
			ctx.Set("DB_type", "UNDEFINED")
		}
	}

	// Raw config vars
	ctx.Set("testBase", c.TestBase)
	ctx.Set("flags", c.Flags)
	ctx.Set("parent_guid", c.ParentGUID)
	ctx.Set("envName", c.EnvName)
	ctx.Set("systemID", ctx.SystemID)
}

// splitHostPort splits "ip:port" into (ip, port).
func splitHostPort(addr string) (string, string) {
	colonIdx := strings.LastIndex(addr, ":")
	if colonIdx < 0 {
		return addr, ""
	}
	return addr[:colonIdx], addr[colonIdx+1:]
}

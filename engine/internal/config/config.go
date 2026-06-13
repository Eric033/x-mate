package config

import (
	"fmt"
	"strings"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// Config holds all parsed command-line configuration.
type Config struct {
	TestBase      string
	Server        string
	Flags         string
	Verbose       bool
	DryRun        bool
	DBInfo        string
	DamperServer  string
	EnvName       string
	ParentGUID    string
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Flags:      "core",
		EnvName:    "UNDEFINED",
		ParentGUID: "92508788-4c1c-11e9-808b-005056a01111",
	}
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

// InitContext populates a TestContext from Config.
func (c *Config) InitContext(ctx *context.TestContext) {
	ctx.TestBase = c.TestBase
	ctx.Flags = c.Flags
	ctx.Verbose = c.Verbose
	ctx.DryRun = c.DryRun
	ctx.EnvName = c.EnvName
	ctx.ParentGUID = c.ParentGUID

	// Servers
	ctx.Servers = ParseServer(c.Server)
	for i, s := range ctx.Servers {
		ctx.Set(fmt.Sprintf("server_%d", i+1), s.IP)
		ctx.Set(fmt.Sprintf("port_%d", i+1), s.Port)
	}

	// Damper
	tcpAddr, httpAddr := ParseDamper(c.DamperServer)
	ctx.DamperTCP = tcpAddr
	ctx.DamperHTTP = httpAddr

	// Parse damper into individual vars
	damParts := strings.Split(c.DamperServer, ":")
	if len(damParts) >= 4 {
		ctx.Set("tcpDamServerIP", damParts[0])
		ctx.Set("tcpDamServerPort", damParts[2])
		ctx.Set("httpDamServerIP", damParts[0])
		ctx.Set("httpDamServerPort", damParts[3])
	}

	// DB
	ctx.DBPools = ParseDBInfo(c.DBInfo)
	for i, db := range ctx.DBPools {
		ctx.Set(fmt.Sprintf("dbips_%d", i+1), db.IP)
		ctx.Set(fmt.Sprintf("dbports_%d", i+1), db.Port)
		ctx.Set(fmt.Sprintf("dbnames_%d", i+1), db.Name)
	}

	// Raw config vars
	ctx.Set("testBase", c.TestBase)
	ctx.Set("flags", c.Flags)
	ctx.Set("G_server", c.Server)
	ctx.Set("G_DamplerServer", c.DamperServer)
	ctx.Set("parent_guid", c.ParentGUID)
	ctx.Set("DB_info", c.DBInfo)
	ctx.Set("envName", c.EnvName)

	if len(ctx.DBPools) > 0 {
		db0 := ctx.DBPools[0]
		ctx.Set("DB_IP", db0.IP)
		ctx.Set("DB_port", db0.Port)
		ctx.Set("DB_name", db0.Name)
		ctx.Set("DB_user", db0.User)
		ctx.Set("DB_passwd", db0.Passwd)
		ctx.Set("DB_type", db0.Type)
	} else {
		ctx.Set("DB_IP", "")
		ctx.Set("DB_port", "")
		ctx.Set("DB_name", "")
		ctx.Set("DB_user", "readonly")
		ctx.Set("DB_passwd", "readonly")
		ctx.Set("DB_type", "UNDEFINED")
	}
}

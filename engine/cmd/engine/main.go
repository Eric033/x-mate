package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Eric033/x-mate/engine/internal/config"
	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/report"
	"github.com/Eric033/x-mate/engine/internal/runner"
	"github.com/Eric033/x-mate/engine/handlers/damper"
	httpHandler "github.com/Eric033/x-mate/engine/handlers/http"
	"github.com/Eric033/x-mate/engine/handlers/rsa"
	"github.com/Eric033/x-mate/engine/handlers/runtime"
	"github.com/Eric033/x-mate/engine/handlers/tcp"
)

func main() {
	cfg := config.Default()

	fs := flag.NewFlagSet("engine", flag.ExitOnError)
	fs.StringVar(&cfg.TestBase, "test-base", "", "Test case root directory (required)")
	fs.StringVar(&cfg.Server, "server", "", "Target server address ip:port or ip1:port1@ip2:port2 (required)")
	fs.StringVar(&cfg.Flags, "flags", "core", "Case tag filter")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose logging")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Validate only, do not execute")
	fs.StringVar(&cfg.DBInfo, "db-info", "", "Database connection info ip:port:dbname:user:passwd")
	fs.StringVar(&cfg.DamperServer, "damper-server", "", "Damper server address ip:port:tcpPort:httpPort")
	fs.StringVar(&cfg.EnvName, "env-name", "UNDEFINED", "Environment name")
	fs.StringVar(&cfg.ParentGUID, "parent-guid", "92508788-4c1c-11e9-808b-005056a01111", "Parent GUID")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	if cfg.TestBase == "" || cfg.Server == "" {
		fmt.Fprintln(os.Stderr, "Error: --test-base and --server are required")
		fmt.Fprintln(os.Stderr)
		fs.Usage()
		os.Exit(1)
	}

	// Initialize context
	ctx := context.New()
	cfg.InitContext(ctx)

	// Initialize handler registry
	registry := handler.NewRegistry()
	registerHandlers(registry, cfg)

	// Create runner
	r := runner.NewRunner(registry)
	r.Logger = func(format string, args ...interface{}) {
		log.Printf(format, args...)
	}

	// Run tests
	log.Printf("Engine starting — testBase=%s server=%s flags=%s", cfg.TestBase, cfg.Server, cfg.Flags)
	result := r.Run(ctx)

	// Print report
	report.PrintReport(os.Stdout, result)

	if result.FailedCases > 0 {
		os.Exit(1)
	}
}

// registerHandlers registers all built-in step type handlers.
func registerHandlers(reg *handler.Registry, cfg *config.Config) {
	// TCP handlers
	reg.Register("xml_set_8", &tcp.XMLSet8Handler{})
	reg.Register("xml_set_sas", &tcp.XMLSetSASHandler{})
	reg.Register("xml_set", &tcp.XMLSetHandler{})
	reg.Register("mca", &tcp.MCAHandler{})

	// HTTP handlers
	reg.Register("http", &httpHandler.HTTPHandler{UseDamper: false})
	reg.Register("damper_set", &httpHandler.HTTPHandler{UseDamper: true})

	// SQL handlers (initialized lazily when DB is configured)
	if cfg.DBInfo != "" && cfg.DBInfo != "UNDEFINED" {
		// DB pool initialization would go here
		// For now, the SQL handlers accept nil pool and will error gracefully
	}

	// Damper handlers
	reg.Register("tcp_damper_set", &damper.TCPDamperSetHandler{})
	reg.Register("mca_damper_set", &damper.MCADamperSetHandler{})

	// Runtime handler
	reg.Register("runtime_verify", &runtime.RuntimeVerifyHandler{})

	// RSA handler
	reg.Register("rsa", &rsa.RSAHandler{})
}

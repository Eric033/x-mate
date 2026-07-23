package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/Eric033/x-mate/engine/internal/config"
	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/report"
	"github.com/Eric033/x-mate/engine/internal/runner"
	"github.com/Eric033/x-mate/engine/internal/sampler"
	"github.com/Eric033/x-mate/engine/handlers/damper"
	httpHandler "github.com/Eric033/x-mate/engine/handlers/http"
	"github.com/Eric033/x-mate/engine/handlers/rsa"
	"github.com/Eric033/x-mate/engine/handlers/runtime"
	"github.com/Eric033/x-mate/engine/handlers/tcp"
)

func main() {
	cfg := config.Default()

	fs := flag.NewFlagSet("engine", flag.ExitOnError)
	fs.StringVar(&cfg.ConfigPath, "config", "", "Path to YAML config file")
	fs.StringVar(&cfg.ActiveProfile, "profile", "default", "Active profile for multi-env config")
	fs.StringVar(&cfg.TestBase, "test-base", "", "Test case root directory (required)")
	fs.StringVar(&cfg.Server, "server", "", "Target server address ip:port or ip1:port1@ip2:port2")
	fs.StringVar(&cfg.Flags, "flags", "core", "Case tag filter")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose logging")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Validate only, do not execute")
	fs.StringVar(&cfg.DBInfo, "db-info", "", "Database connection info ip:port:dbname:user:passwd")
	fs.StringVar(&cfg.DamperServer, "damper-server", "", "Damper server address ip:port:tcpPort:httpPort")
	fs.StringVar(&cfg.EnvName, "env-name", "UNDEFINED", "Environment name")
	fs.StringVar(&cfg.ParentGUID, "parent-guid", "92508788-4c1c-11e9-808b-005056a01111", "Parent GUID")
	var runAll bool
	fs.BoolVar(&runAll, "run-all", false, "Run all cases, ignore flags filtering")
	fs.IntVar(&cfg.Concurrency, "concurrency", 1, "Max concurrent test cases (1 = serial)")

	var startMock bool
	fs.BoolVar(&startMock, "start-mock", false, "Start built-in mock HTTP server before running tests")

	var reportFile string
	fs.StringVar(&reportFile, "report-file", "", "Save test report to file")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	// Start mock server if requested
	if startMock {
		go startMockServer()
		// Give it a moment to bind
		time.Sleep(200 * time.Millisecond)
		log.Printf("Mock server started on :19876")
	}

	// Load YAML config (if found)
	if err := cfg.LoadYAML(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config load: %v\n", err)
	}

	if cfg.TestBase == "" {
		fmt.Fprintln(os.Stderr, "Error: --test-base is required")
		fmt.Fprintln(os.Stderr)
		fs.Usage()
		os.Exit(1)
	}

	// If --report-file is set, tee all output to file as well
	var reportWriter io.Writer = os.Stdout
	if reportFile != "" {
		f, err := os.Create(reportFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot create report file %s: %v\n", reportFile, err)
			os.Exit(1)
		}
		defer f.Close()
		// Tee log output (stderr) to file
		log.SetOutput(io.MultiWriter(os.Stderr, f))
		// Tee report output (stdout) to file
		reportWriter = io.MultiWriter(os.Stdout, f)
	}

	// Initialize context
	ctx := context.New()
	cfg.InitContext(ctx)
	ctx.RunAll = runAll

	// Initialize handler registry
	registry := handler.NewRegistry()
	registerHandlers(registry, cfg)

	// Create runner
	r := runner.NewRunner(registry)
	r.Logger = func(format string, args ...interface{}) {
		log.Printf(format, args...)
	}

	// Run tests
	log.Printf("Engine starting — testBase=%s flags=%s profile=%s", cfg.TestBase, cfg.Flags, cfg.ActiveProfile)
	result, runErr := r.Run(ctx)

	// Print report
	report.PrintReport(reportWriter, result)

	if result.FailedCases > 0 || result.ErrorCases > 0 || runErr != nil {
		os.Exit(1)
	}
}

// startMockServer launches a lightweight HTTP mock in a goroutine.
// It simulates a simple order management backend for demo purposes.
func startMockServer() {
	mux := http.NewServeMux()
	var orderCounter int64

	mux.HandleFunc("/api/order/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			TranCode string `json:"tran_code"`
			Amount   string `json:"amount"`
			UserID   string `json:"user_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		orderID := atomic.AddInt64(&orderCounter, 1)
		resp := map[string]interface{}{
			"ret_code":  "000000",
			"ret_msg":   "success",
			"order_id":  fmt.Sprintf("ORD%08d", orderID),
			"tran_code": req.TranCode,
			"amount":    req.Amount,
			"status":    "ACTIVE",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/order/query", func(w http.ResponseWriter, r *http.Request) {
		orderID := r.URL.Query().Get("order_id")
		if orderID == "" {
			orderID = r.URL.Query().Get("orderId")
		}
		resp := map[string]interface{}{
			"ret_code": "000000",
			"ret_msg":  "success",
			"order_id": orderID,
			"status":   "ACTIVE",
			"amount":   "10000",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Fatal(http.ListenAndServe(":19876", mux))
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

	// SQL handlers (initialized lazily when services have DB configured)
	dbManager := sampler.NewDBPoolManager()
	if len(cfg.Services) > 0 {
		// DB pool initialization would go here with the real driver
		// For now, the SQL handlers accept nil pool and will error gracefully
		_ = dbManager
	}

	// Damper handlers
	reg.Register("tcp_damper_set", &damper.TCPDamperSetHandler{})
	reg.Register("mca_damper_set", &damper.MCADamperSetHandler{})

	// Runtime handler
	reg.Register("runtime_verify", &runtime.RuntimeVerifyHandler{})

	// RSA handler
	reg.Register("rsa", &rsa.RSAHandler{})
}
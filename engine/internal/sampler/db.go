package sampler

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// DBPoolManager manages database connection pools.
type DBPoolManager struct {
	mu    sync.Mutex
	pools map[string]*sql.DB // keyed by service name (e.g. "ABC")
}

// NewDBPoolManager creates a new DB pool manager.
func NewDBPoolManager() *DBPoolManager {
	return &DBPoolManager{pools: make(map[string]*sql.DB)}
}

// Init creates connection pools from service definitions.
// driver can be "oracle", "mysql", "postgres" etc.
func (m *DBPoolManager) Init(services map[string]context.ServiceDef, driver string) error {
	for name, svc := range services {
		if svc.DB == nil {
			continue
		}
		cfg := DBConfig{
			IP:     svc.DB.Address,
			Port:   "",
			Name:   svc.DB.Database,
			User:   svc.DB.Username,
			Passwd: svc.DB.Password,
			Type:   "UNDEFINED",
		}
		// Split address into IP:Port
		if idx := strings.LastIndex(svc.DB.Address, ":"); idx >= 0 {
			cfg.IP = svc.DB.Address[:idx]
			cfg.Port = svc.DB.Address[idx+1:]
		}

		dsn := buildDSN(cfg, driver)
		db, err := sql.Open(driver, dsn)
		if err != nil {
			return fmt.Errorf("db pool %s init: %w", name, err)
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Second)
		db.SetConnMaxIdleTime(60 * time.Second)
		m.pools[name] = db
	}
	return nil
}

// Get returns the DB pool for a given service name.
func (m *DBPoolManager) Get(serviceName string) (*sql.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	db, ok := m.pools[serviceName]
	if !ok {
		return nil, fmt.Errorf("no db pool for service %s", serviceName)
	}
	return db, nil
}

// RegisterPool adds a pre-opened *sql.DB to the pool manager for the given name.
// This is useful for tests that want to inject a SQLite in-memory database.
func (m *DBPoolManager) RegisterPool(name string, db *sql.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pools[name] = db
}

// AddPool opens and registers a database connection pool for a named service.
// It generates the DSN from the provided parameters and performs a fail-fast
// connectivity check via Ping. Supported driver types: sqlite3, oracle, mysql.
func (m *DBPoolManager) AddPool(serviceName, driverType, address, database, username, password string) error {
	var dsn string
	switch strings.ToLower(driverType) {
	case "sqlite3":
		dsn = database
	case "oracle":
		if strings.Contains(address, ":") {
			dsn = fmt.Sprintf("oracle://%s:%s@%s/%s", username, password, address, database)
		} else {
			dsn = fmt.Sprintf("oracle://%s:%s@%s:1521/%s", username, password, address, database)
		}
	case "mysql":
		if strings.Contains(address, ":") {
			dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s", username, password, address, database)
		} else {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", username, password, address, database)
		}
	default:
		return fmt.Errorf("unsupported database type: %s", driverType)
	}

	db, err := sql.Open(driverType, dsn)
	if err != nil {
		return fmt.Errorf("open %s: %w", driverType, err)
	}

	// Test connection (fail-fast)
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("ping %s: %w", driverType, err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Second)
	db.SetConnMaxIdleTime(60 * time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pools[serviceName] = db
	return nil
}

// Close closes all pools.
func (m *DBPoolManager) Close() {
	for _, db := range m.pools {
		db.Close()
	}
}

// DBConfig represents a database connection config.
type DBConfig struct {
	IP     string
	Port   string
	Name   string
	User   string
	Passwd string
	Type   string
}

// buildDSN builds a data source name string.
func buildDSN(cfg DBConfig, driver string) string {
	switch driver {
	case "oracle":
		return fmt.Sprintf("oracle://%s:%s@%s:%s/%s", cfg.User, cfg.Passwd, cfg.IP, cfg.Port, cfg.Name)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cfg.User, cfg.Passwd, cfg.IP, cfg.Port, cfg.Name)
	case "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			cfg.IP, cfg.Port, cfg.User, cfg.Passwd, cfg.Name)
	case "sqlite3":
		// SQLite: address is file path or ":memory:"
		return cfg.IP
	default:
		return fmt.Sprintf("%s:%s@%s:%s/%s", cfg.User, cfg.Passwd, cfg.IP, cfg.Port, cfg.Name)
	}
}

// QueryResult represents a row from a SQL query.
type QueryResult struct {
	Columns []string
	Rows    []map[string]string
}

// Select executes a SELECT query and returns rows as maps.
func (m *DBPoolManager) Select(serviceName string, query string, args ...interface{}) (*QueryResult, error) {
	db, err := m.Get(serviceName)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("sql query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("sql columns: %w", err)
	}

	result := &QueryResult{Columns: cols}
	for rows.Next() {
		values := make([]sql.NullString, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("sql scan: %w", err)
		}
		row := make(map[string]string)
		for i, col := range cols {
			if values[i].Valid {
				row[col] = values[i].String
			} else {
				row[col] = ""
			}
		}
		result.Rows = append(result.Rows, row)
	}

	return result, nil
}

// Exec executes an UPDATE/INSERT/DELETE and returns rows affected.
func (m *DBPoolManager) Exec(serviceName string, query string, args ...interface{}) (int64, error) {
	db, err := m.Get(serviceName)
	if err != nil {
		return 0, err
	}

	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("sql exec: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sql rows affected: %w", err)
	}

	return affected, nil
}

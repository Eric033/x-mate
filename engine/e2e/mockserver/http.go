// Package mockserver provides enhanced HTTP and TCP mock servers for E2E testing.
package mockserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPMock is an HTTP mock server for Engine E2E testing.
type HTTPMock struct {
	Server       *http.Server
	Listener     string
	listener     net.Listener
	orderCounter int64
	orders       []map[string]interface{}
	mu           sync.RWMutex
}

// NewHTTPMock creates and starts an HTTP mock server on a random port.
func NewHTTPMock() (*HTTPMock, error) {
	mux := http.NewServeMux()
	mock := &HTTPMock{}

	// POST /api/order/create
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

		oid := atomic.AddInt64(&mock.orderCounter, 1)
		order := map[string]interface{}{
			"ret_code":  "000000",
			"ret_msg":   "success",
			"order_id":  fmt.Sprintf("ORD%08d", oid),
			"tran_code": req.TranCode,
			"amount":    req.Amount,
			"user_id":   req.UserID,
			"status":    "ACTIVE",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		}
		mock.mu.Lock()
		mock.orders = append(mock.orders, order)
		mock.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(order)
	})

	// GET /api/order/query
	mux.HandleFunc("/api/order/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		orderID := r.URL.Query().Get("order_id")
		if orderID == "" {
			orderID = r.URL.Query().Get("orderId")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ret_code": "000000",
			"ret_msg":  "success",
			"order_id": orderID,
			"status":   "ACTIVE",
			"amount":   "10000",
		})
	})

	// GET /health
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// POST /api/order/cancel
	mux.HandleFunc("/api/order/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			OrderID string `json:"order_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ret_code": "000000",
			"ret_msg":  "cancelled",
			"order_id": req.OrderID,
			"status":   "CANCELLED",
		})
	})

	// GET /api/order/list (tests JSONPath array)
	mux.HandleFunc("/api/order/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		mock.mu.RLock()
		list := mock.orders
		mock.mu.RUnlock()
		if list == nil {
			list = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ret_code": "000000",
			"ret_msg":  "success",
			"total":    len(list),
			"orders":   list,
		})
	})

	// POST /api/echo
	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		w.Write(body[:n])
	})

	// GET /api/error/500
	mux.HandleFunc("/api/error/500", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error","code":"ERR500"}`))
	})

	// GET /api/error/timeout
	mux.HandleFunc("/api/error/timeout", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"too late"}`))
	})

	// GET /api/error/malformed
	mux.HandleFunc("/api/error/malformed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("http mock listen: %w", err)
	}
	mock.listener = ln
	mock.Listener = ln.Addr().String()
	mock.Server = &http.Server{Handler: mux}

	go func() {
		if err := mock.Server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP mock server error: %v", err)
		}
	}()

	return mock, nil
}

// Close shuts down the HTTP mock server.
func (m *HTTPMock) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}

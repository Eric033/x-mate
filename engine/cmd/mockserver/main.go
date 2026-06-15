package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// MockServer is a lightweight HTTP mock for Engine demos.
// It simulates a simple order management backend.
func main() {
	mux := http.NewServeMux()
	var orderCounter int64

	// POST /api/order/create — create an order
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Allow form data too
			req.TranCode = r.FormValue("tran_code")
			req.Amount = r.FormValue("amount")
			req.UserID = r.FormValue("user_id")
		}

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

	// GET /api/order/query — query an order by order_id
	mux.HandleFunc("/api/order/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

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

	// GET /health — health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":19876"
	log.Printf("Mock server listening on %s", addr)
	log.Printf("  POST /api/order/create  — create order")
	log.Printf("  GET  /api/order/query   — query order")
	log.Printf("  GET  /health            — health check")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("mock server failed: %v", err)
	}
}

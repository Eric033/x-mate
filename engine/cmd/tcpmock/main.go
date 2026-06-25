package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// stripBCD strips a BCD length prefix from the data if present.
func stripBCD(data []byte) []byte {
	if len(data) < 6 {
		return data
	}
	allDigits := true
	for i := 0; i < 8 && i < len(data); i++ {
		if data[i] < '0' || data[i] > '9' {
			allDigits = false
			break
		}
	}
	if !allDigits {
		return data
	}
	// Try 8-byte prefix first
	if len(data) > 8 {
		if n, err := strconv.Atoi(string(data[:8])); err == nil && n > 0 && n <= len(data)-8 {
			return data[8:]
		}
	}
	// Fallback to 6-byte
	if len(data) > 6 {
		if n, err := strconv.Atoi(string(data[:6])); err == nil && n > 0 && n <= len(data)-6 {
			return data[6:]
		}
	}
	return data
}

// handleConnection processes a single TCP connection — mirror echo.
func handleConnection(conn net.Conn, port int) {
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Read all data
	var buf []byte
	tmp := make([]byte, 65536)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && len(buf) > 0 {
				break
			}
			if len(buf) == 0 {
				log.Printf("  [port %d] read error: %v", port, err)
				return
			}
			break
		}
		// If we got a complete XML (ends with '>'), break
		if n > 0 && buf[len(buf)-1] == '>' {
			break
		}
	}

	if len(buf) == 0 {
		return
	}

	log.Printf("  [port %d] received %d bytes", port, len(buf))

	// Strip BCD prefix to get the raw XML
	xmlBytes := stripBCD(buf)
	xmlStr := string(xmlBytes)
	log.Printf("  [port %d] response XML: %s...", port, xmlStr[:min(len(xmlStr), 80)])

	// Mirror: wrap the raw XML with 6-byte BCD length prefix
	prefix := fmt.Sprintf("%06d", len(xmlStr))
	payload := append([]byte(prefix), xmlBytes...)

	// Send response
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Write(payload)
	if err != nil {
		log.Printf("  [port %d] write error: %v", port, err)
		return
	}
	log.Printf("  [port %d] sent %d bytes (mirror response)", port, n)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// startTCPServer listens on a single port.
func startTCPServer(wg *sync.WaitGroup, port int) {
	defer wg.Done()

	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("ERROR: cannot listen on port %d: %v", port, err)
		return
	}
	defer listener.Close()

	log.Printf("TCP mock (mirror) server listening on port %d", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("  [port %d] accept error: %v", port, err)
			return
		}
		go handleConnection(conn, port)
	}
}

func main() {
	// Parse ports from args or use defaults
	ports := []int{50500}
	if len(os.Args) > 1 {
		ports = nil
		for _, arg := range os.Args[1:] {
			p, err := strconv.Atoi(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid port: %s\n", arg)
				os.Exit(1)
			}
			ports = append(ports, p)
		}
	}

	log.Printf("Starting TCP mirror mock server on ports: %v", ports)

	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go startTCPServer(&wg, port)
	}

	log.Println("TCP mirror mock servers started. Press Ctrl+C to stop.")
	wg.Wait()
}

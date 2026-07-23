package mockserver

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// TCPMock is a TCP mock server that simulates the BCD-length-prefix protocol.
// It auto-detects the request format and responds accordingly:
//   - If request ends with \r\n, responds with MCA format (XML + \r\n)
//   - If request starts with 8 ASCII digits, responds with xml_set_8 format (8-byte prefix + '>')
//   - Otherwise responds with xml_set format (6-byte prefix + '>')
type TCPMock struct {
	Listener   string
	listener   net.Listener
	closed     bool
	mu         sync.Mutex
	requests   [][]byte
}

// NewTCPMock creates and starts a TCP mock server on a random port.
func NewTCPMock() (*TCPMock, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("tcp mock listen: %w", err)
	}

	mock := &TCPMock{
		listener: ln,
		Listener: ln.Addr().String(),
	}

	go mock.acceptLoop()
	return mock, nil
}

// ResponseXML is the canned response XML used by the TCP mock.
var ResponseXML = `<Response><Header><TRAN_CODE>TRAN001</TRAN_CODE><RESP_CODE>000000</RESP_CODE><RESP_MSG>SUCCESS</RESP_MSG></Header><Body><AMOUNT>100</AMOUNT><BALANCE>50000</BALANCE></Body></Response>`

func (m *TCPMock) acceptLoop() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			if !m.isClosed() {
				log.Printf("TCP mock accept error: %v", err)
			}
			return
		}
		go m.handleConn(conn)
	}
}

func (m *TCPMock) handleConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Read all data using idle timeout detection.
	// After receiving any data, set a short (200ms) idle timeout.
	// If no more data within that window, assume request is complete.
	var buf bytes.Buffer
	readBuf := make([]byte, 4096)
	idleTimeout := false

	for {
		n, err := conn.Read(readBuf)
		if n > 0 {
			buf.Write(readBuf[:n])
			// Got data; next read will use a short idle timeout
			conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
			idleTimeout = true
			continue
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			// Timeout or other error
			if idleTimeout && buf.Len() > 0 {
				break // idle timeout, we have all data
			}
			return // no data at all
		}
	}

	data := buf.Bytes()

	// Store request for inspection in tests
	m.mu.Lock()
	m.requests = append(m.requests, data)
	m.mu.Unlock()

	// Determine response format based on request
	var response []byte
	respXML := []byte(ResponseXML)

	switch {
	case bytes.HasSuffix(data, []byte("\r\n")):
		// MCA format: return XML + \r\n
		response = make([]byte, len(respXML)+2)
		copy(response, respXML)
		response[len(respXML)] = '\r'
		response[len(respXML)+1] = '\n'

	case len(data) >= 8 && isAllDigits(data[:8]):
		// xml_set_8: 8-byte BCD prefix + XML + '>'
		prefix := fmt.Sprintf("%08d", len(respXML))
		response = make([]byte, 8+len(respXML)+1)
		copy(response, prefix)
		copy(response[8:], respXML)
		response[8+len(respXML)] = '>'

	default:
		// xml_set: 6-byte BCD prefix + XML + '>'
		prefix := fmt.Sprintf("%06d", len(respXML))
		response = make([]byte, 6+len(respXML)+1)
		copy(response, prefix)
		copy(response[6:], respXML)
		response[6+len(respXML)] = '>'
	}

	// Reset deadline before writing (previously set by read idle timeout)
	conn.SetDeadline(time.Time{})
	conn.Write(response)
}

// Requests returns a copy of all received requests.
func (m *TCPMock) Requests() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([][]byte, len(m.requests))
	for i, r := range m.requests {
		cp[i] = make([]byte, len(r))
		copy(cp[i], r)
	}
	return cp
}

// Close shuts down the TCP mock server.
func (m *TCPMock) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	if m.listener != nil {
		m.listener.Close()
	}
}

func (m *TCPMock) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// isAllDigits checks if all bytes are ASCII digits.
func isAllDigits(b []byte) bool {
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

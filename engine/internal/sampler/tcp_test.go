package sampler

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// ---- BuildBCDLengthPrefix tests ----

func TestBuildBCDLengthPrefix_Normal(t *testing.T) {
	prefix := BuildBCDLengthPrefix(1234, 8)
	want := []byte("00001234")
	if !bytes.Equal(prefix, want) {
		t.Errorf("got %v, want %v", prefix, want)
	}
}

func TestBuildBCDLengthPrefix_LargeValue(t *testing.T) {
	prefix := BuildBCDLengthPrefix(99999999, 8)
	want := []byte("99999999")
	if !bytes.Equal(prefix, want) {
		t.Errorf("got %v, want %v", prefix, want)
	}
}

func TestBuildBCDLengthPrefix_Zero(t *testing.T) {
	prefix := BuildBCDLengthPrefix(0, 8)
	want := []byte("00000000")
	if !bytes.Equal(prefix, want) {
		t.Errorf("got %v, want %v", prefix, want)
	}
}

// ---- EncodeBCDLength tests ----

func TestEncodeBCDLength_Normal(t *testing.T) {
	b := EncodeBCDLength(1234, 2)
	// 1234 → bytes: high nibble *10 + low nibble per byte
	// byte[0]: 0x12, byte[1]: 0x34
	if len(b) != 2 {
		t.Fatalf("len = %d, want 2", len(b))
	}
	if b[0] != 0x12 {
		t.Errorf("b[0] = 0x%02x, want 0x12", b[0])
	}
	if b[1] != 0x34 {
		t.Errorf("b[1] = 0x%02x, want 0x34", b[1])
	}
}

func TestEncodeBCDLength_OddDigits(t *testing.T) {
	b := EncodeBCDLength(7, 2)
	// 7 → 0x00 0x07
	if b[0] != 0x00 || b[1] != 0x07 {
		t.Errorf("got %x, want [0x00 0x07]", b)
	}
}

func TestEncodeBCDLength_MaxValue(t *testing.T) {
	b := EncodeBCDLength(9999, 2)
	if b[0] != 0x99 || b[1] != 0x99 {
		t.Errorf("got %x, want [0x99 0x99]", b)
	}
}

// ---- ParseBCDLength tests ----

func TestParseBCDLength_Normal(t *testing.T) {
	data := []byte{0x12, 0x34}
	got := ParseBCDLength(data)
	if got != 1234 {
		t.Errorf("got %d, want 1234", got)
	}
}

func TestParseBCDLength_AllZeros(t *testing.T) {
	data := []byte{0x00, 0x00}
	got := ParseBCDLength(data)
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestParseBCDLength_Max(t *testing.T) {
	data := []byte{0x99, 0x99}
	got := ParseBCDLength(data)
	if got != 9999 {
		t.Errorf("got %d, want 9999", got)
	}
}

func TestParseBCDLength_SingleByte(t *testing.T) {
	data := []byte{0x45}
	got := ParseBCDLength(data)
	if got != 45 {
		t.Errorf("got %d, want 45", got)
	}
}

// ---- EncodeBCDLength / ParseBCDLength round-trip ----

func TestBCDEncodeDecode_RoundTrip(t *testing.T) {
	inputs := []int{0, 1, 12, 123, 1234, 9999}
	for _, n := range inputs {
		encoded := EncodeBCDLength(n, 2)
		decoded := ParseBCDLength(encoded)
		if decoded != n {
			t.Errorf("round-trip failed: %d → %x → %d", n, encoded, decoded)
		}
	}
}

// ---- DefaultTCPConfig ----

func TestDefaultTCPConfig(t *testing.T) {
	cfg := DefaultTCPConfig()
	if cfg.ConnectTimeout != 3*time.Second {
		t.Errorf("ConnectTimeout = %v, want 3s", cfg.ConnectTimeout)
	}
	if cfg.ReadTimeout != 60*time.Second {
		t.Errorf("ReadTimeout = %v, want 60s", cfg.ReadTimeout)
	}
	if cfg.EolByte != 0 {
		t.Errorf("EolByte = %d, want 0", cfg.EolByte)
	}
	if cfg.ReUseConnection != false {
		t.Errorf("ReUseConnection should be false by default")
	}
}

// ---- TCPSend tests (local echo server) ----

func TestTCPSend_Echo(t *testing.T) {
	// Start a local TCP echo server
	echoAddr := startEchoServer(t)

	payload := []byte("hello tcp")
	cfg := TCPConfig{
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    5 * time.Second,
		EolByte:        0,
	}

	resp, err := TCPSend(echoAddr, payload, cfg)
	if err != nil {
		t.Fatalf("TCPSend failed: %v", err)
	}

	if !bytes.Equal(resp, payload) {
		t.Errorf("response = %q, want %q", string(resp), string(payload))
	}
}

func TestTCPSend_MultiLine(t *testing.T) {
	echoAddr := startEchoServer(t)

	payload := []byte("line1\nline2\nline3\n")
	cfg := TCPConfig{
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    5 * time.Second,
	}

	resp, err := TCPSend(echoAddr, payload, cfg)
	if err != nil {
		t.Fatalf("TCPSend failed: %v", err)
	}

	if !bytes.Equal(resp, payload) {
		t.Errorf("response mismatch: got %q, want %q", string(resp), string(payload))
	}
}

func TestTCPSend_EolByte(t *testing.T) {
	// Start an echo server that will echo "hello\n"
	echoAddr := startEchoServer(t)

	payload := []byte("hello\n")
	cfg := TCPConfig{
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    5 * time.Second,
		EolByte:        '\n',
	}

	resp, err := TCPSend(echoAddr, payload, cfg)
	if err != nil {
		t.Fatalf("TCPSend failed: %v", err)
	}

	if string(resp) != "hello\n" {
		t.Errorf("response = %q, want %q", string(resp), "hello\n")
	}
}

func TestTCPSend_ConnectionRefused(t *testing.T) {
	// An address we know nothing is listening on
	cfg := TCPConfig{
		ConnectTimeout: 1 * time.Second,
		ReadTimeout:    1 * time.Second,
	}

	_, err := TCPSend("127.0.0.1:1", []byte("test"), cfg)
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
}

func TestTCPSend_CloseConnection(t *testing.T) {
	// Use a simple accept-and-close server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()

	payload := []byte("close test")
	cfg := TCPConfig{
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    5 * time.Second,
		CloseConnection: true,
	}

	resp, err := TCPSend(addr, payload, cfg)
	if err != nil {
		t.Fatalf("TCPSend failed: %v", err)
	}

	if !bytes.Equal(resp, payload) {
		t.Errorf("response = %q, want %q", string(resp), string(payload))
	}

	<-done
}

func TestTCPSend_EmptyPayload(t *testing.T) {
	echoAddr := startEchoServer(t)

	cfg := TCPConfig{
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    5 * time.Second,
	}

	resp, err := TCPSend(echoAddr, []byte{}, cfg)
	if err != nil {
		t.Fatalf("TCPSend failed: %v", err)
	}

	if len(resp) != 0 {
		t.Errorf("expected empty response, got %q", string(resp))
	}
}

// ---- EncodeUint32BE (compile check) ----

func TestEncodeUint32BE_CompileCheck(t *testing.T) {
	// This just checks that binary.BigEndian is accessible via the import
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], 42)
	if buf[3] != 42 {
		t.Errorf("unexpected big-endian encoding")
	}
}

// ---- Helper: startEchoServer ----

func startEchoServer(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	addr := ln.Addr().String()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						c.Write(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	// Clean up when test ends
	t.Cleanup(func() { ln.Close() })

	return addr
}
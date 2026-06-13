package sampler

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// TCPConfig holds TCP connection parameters.
type TCPConfig struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	EolByte        byte // 0 means no EOL checking
	ReUseConnection bool
	CloseConnection bool
}

// DefaultTCPConfig returns the default TCP configuration from SRS.
func DefaultTCPConfig() TCPConfig {
	return TCPConfig{
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    60 * time.Second,
	}
}

// TCPSend connects to addr, sends the payload, and returns the response.
// If prefixLen > 0, prepends a BCD-encoded length prefix of that many bytes.
func TCPSend(addr string, payload []byte, cfg TCPConfig) ([]byte, error) {
	dialer := net.Dialer{Timeout: cfg.ConnectTimeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp connect %s: %w", addr, err)
	}
	defer func() {
		if cfg.CloseConnection || !cfg.ReUseConnection {
			conn.Close()
		}
	}()

	conn.SetDeadline(time.Now().Add(cfg.ReadTimeout))

	// Send
	_, err = conn.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("tcp write: %w", err)
	}

	// Read response
	var buf []byte
	recvBuf := make([]byte, 4096)
	for {
		n, err := conn.Read(recvBuf)
		if n > 0 {
			buf = append(buf, recvBuf[:n]...)
		}
		if cfg.EolByte != 0 && n > 0 && recvBuf[n-1] == cfg.EolByte {
			break
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		// If we have data and connection is closed, break
		if n == 0 {
			break
		}
	}

	return buf, nil
}

// BuildBCDLengthPrefix creates an 8-byte BCD-encoded length prefix.
// The length is formatted as %08d and converted to bytes.
func BuildBCDLengthPrefix(payloadLen int, prefixBytes int) []byte {
	formatStr := fmt.Sprintf("%%0%dd", prefixBytes)
	prefixStr := fmt.Sprintf(formatStr, payloadLen)
	return []byte(prefixStr)
}

// EncodeBCDLength encodes a number as BCD bytes (big-endian decimal digits).
func EncodeBCDLength(length int, numBytes int) []byte {
	buf := make([]byte, numBytes)
	for i := numBytes - 1; i >= 0; i-- {
		buf[i] = byte(length % 10) // low nibble
		length /= 10
		buf[i] |= byte((length%10)<<4) // high nibble
		length /= 10
	}
	return buf
}

// ParseBCDLength decodes BCD bytes back to an integer.
func ParseBCDLength(data []byte) int {
	result := 0
	for _, b := range data {
		result = result*100 + int(b>>4)*10 + int(b&0x0F)
	}
	return result
}

// EncodeUint32BE encodes a uint32 in big-endian byte order.
var _ = binary.BigEndian

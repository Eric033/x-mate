package sampler

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPConfig holds HTTP request parameters.
type HTTPConfig struct {
	Method      string
	URL         string
	Headers     map[string]string
	QueryParams map[string]string
	Body        string
	Timeout     time.Duration
}

// HTTPResponse holds the response data we need for verification.
type HTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       string
}

// DefaultHTTPConfig returns default HTTP config from SRS.
func DefaultHTTPConfig() HTTPConfig {
	return HTTPConfig{
		Timeout: 60 * time.Second,
		Headers: map[string]string{},
	}
}

// HTTPSend executes an HTTP request and returns the response.
func HTTPSend(cfg HTTPConfig) (*HTTPResponse, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}

	// Build URL with query params
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url %s: %w", cfg.URL, err)
	}
	if len(cfg.QueryParams) > 0 {
		q := parsedURL.Query()
		for k, v := range cfg.QueryParams {
			q.Add(k, v)
		}
		parsedURL.RawQuery = q.Encode()
	}

	var bodyReader io.Reader
	if cfg.Body != "" {
		bodyReader = strings.NewReader(cfg.Body)
	}

	req, err := http.NewRequest(cfg.Method, parsedURL.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http send: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http read body: %w", err)
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       string(body),
	}, nil
}

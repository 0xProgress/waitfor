package network

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

const (
	requestTimeout = 5 * time.Second
	maxBodyBytes   = 4096
	maxBodyDisplay = 200
	maxRedirects   = 10
)

var errRedirectLoop = errors.New("redirect loop detected")

// HTTPCondition waits for an HTTP endpoint to return the expected status.
type HTTPCondition struct {
	url      string
	target   string
	method   string
	wantCode int
	headers  http.Header
	client   *http.Client
	warnings []string

	lastBody string
}

type HTTPOptions struct {
	Method   string
	WantCode int
	Insecure bool
	Headers  []string
}

func NewHTTPCondition(rawURL string, opts HTTPOptions) (*HTTPCondition, error) {
	target := rawURL
	canonical := rawURL
	var warnings []string

	if !strings.Contains(canonical, "://") {
		canonical = "http://" + canonical
		warnings = append(warnings, fmt.Sprintf("URL has no scheme; using %s", canonical))
	}

	parsed, err := url.Parse(canonical)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return nil, fmt.Errorf("invalid URL %q: unsupported scheme %q", rawURL, parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid URL %q: missing host", rawURL)
	}

	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}

	headers := http.Header{}
	for _, h := range opts.Headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q: expected \"Key: Value\"", h)
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			return nil, fmt.Errorf("invalid header %q: empty key", h)
		}
		headers.Add(k, v)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errRedirectLoop
			}
			return nil
		},
	}
	if opts.Insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &HTTPCondition{
		url:      parsed.String(),
		target:   target,
		method:   method,
		wantCode: opts.WantCode,
		headers:  headers,
		client:   client,
		warnings: warnings,
	}, nil
}

func (c *HTTPCondition) Kind() string   { return "http" }
func (c *HTTPCondition) Target() string { return c.target }
func (c *HTTPCondition) Describe() string {
	return fmt.Sprintf("HTTP %s %s", c.method, c.target)
}
func (c *HTTPCondition) SuccessMessage() string {
	return fmt.Sprintf("%s %s", c.method, c.target)
}

func (c *HTTPCondition) Warnings() []string { return c.warnings }
func (c *HTTPCondition) LastDetail() (string, string) {
	return "Body (last)", c.lastBody
}

func (c *HTTPCondition) Check(ctx context.Context) (string, bool, error) {
	c.lastBody = ""

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, c.method, c.url, nil)
	if err != nil {
		return err.Error(), false, err
	}
	for k, vs := range c.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return classifyHTTPError(err), false, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	if c.matchesStatus(resp.StatusCode) {
		return resp.Status, true, nil
	}

	c.lastBody = truncateBody(bodyBytes)
	return resp.Status, false, nil
}

func (c *HTTPCondition) matchesStatus(code int) bool {
	if c.wantCode > 0 {
		return code == c.wantCode
	}
	return code >= 200 && code < 300
}

func classifyHTTPError(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, errRedirectLoop) {
		return "redirect loop detected"
	}

	if dnsErr, ok := errors.AsType[*net.DNSError](err); ok {
		return "DNS resolution failed: " + dnsErr.Name
	}

	if certErr, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return "TLS handshake failed: " + certErr.Error()
	}
	if recordErr, ok := errors.AsType[*tls.RecordHeaderError](err); ok {
		msg := recordErr.Msg
		if msg == "" {
			msg = recordErr.Error()
		}
		return "TLS handshake failed: " + msg
	}
	if msg := err.Error(); strings.Contains(msg, "tls:") || strings.Contains(msg, "x509:") {
		return "TLS handshake failed: " + msg
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused"
	}
	if errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) {
		return "no route to host"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "connection timeout"
	}
	if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		return "connection timeout"
	}

	return err.Error()
}

func truncateBody(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > maxBodyDisplay {
		s = s[:maxBodyDisplay] + "…"
	}
	return s
}

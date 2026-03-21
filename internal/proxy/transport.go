package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// Chrome TLS transport using uTLS to mimic Chrome's JA3/JA4 fingerprint.
// Uses http2.Transport for proper HTTP/2 support with custom TLS dial.
var (
	chromeRoundTripper     http.RoundTripper
	chromeRoundTripperOnce sync.Once
)

func getChromeRoundTripper() http.RoundTripper {
	chromeRoundTripperOnce.Do(func() {
		chromeRoundTripper = &http2.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return dialChromeTLS(ctx, network, addr)
			},
			DisableCompression: true, // We set Accept-Encoding ourselves
		}
	})
	return chromeRoundTripper
}

func dialChromeTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	// Parse host for SNI
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	// Dial raw TCP with context support
	dialer := &net.Dialer{Timeout: AppConfig.TLSDialTimeoutDuration()}
	rawConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial: %w", err)
	}

	// Create uTLS connection with Chrome fingerprint + ALPN h2
	tlsConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		NextProtos:         []string{"h2", "http/1.1"},
	}

	tlsConn := utls.UClient(rawConn, tlsConfig, utls.HelloChrome_Auto)

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("tls handshake: %w", err)
	}

	return tlsConn, nil
}

// getChromeHTTPClient returns an http.Client with Chrome TLS fingerprint
func getChromeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: getChromeRoundTripper(),
		Timeout:   timeout,
	}
}

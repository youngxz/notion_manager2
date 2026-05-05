package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"

	"notion-manager/internal/netutil"
)

// initAccountEnvironment sets up a dedicated, isolated transport and headers for the account.
func (acc *Account) initAccountEnvironment() {
	acc.mu.Lock()
	defer acc.mu.Unlock()

	// Only initialize if not already done
	if acc.HTTPTransport != nil && acc.UserAgent != "" {
		return
	}

	profile, fullVer, majorVer := netutil.GetRandomChromeProfile()

	acc.UserAgent = netutil.GenerateUserAgent(fullVer)
	acc.SecChUa = netutil.GenerateSecChUa(majorVer)
	acc.TLSProfile = fullVer

	// Build a unique transport specifically for this account to prevent multiplexing
	// with other accounts over the same TLS/TCP connection.
	h2Transport := &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}

			dialCtx := ctx
			if to := AppConfig.TLSDialTimeoutDuration(); to > 0 {
				var cancel context.CancelFunc
				dialCtx, cancel = context.WithTimeout(ctx, to)
				defer cancel()
			}

			header := make(http.Header)
			header.Set("User-Agent", netutil.GenerateUserAgent(fullVer))
			header.Set("sec-ch-ua", netutil.GenerateSecChUa(majorVer))
			header.Set("sec-ch-ua-mobile", "?0")
			header.Set("sec-ch-ua-platform", "\"Windows\"")
			rawConn, err := netutil.DialThroughProxy(dialCtx, network, addr, AppConfig.NotionProxyURL(), header)
			if err != nil {
				return nil, fmt.Errorf("tcp dial: %w", err)
			}

			tlsConfig := &utls.Config{
				ServerName:         host,
				InsecureSkipVerify: false,
				MinVersion:         tls.VersionTLS12,
				NextProtos:         []string{"h2", "http/1.1"},
			}

			tlsConn := utls.UClient(rawConn, tlsConfig, profile)

			if err := tlsConn.HandshakeContext(dialCtx); err != nil {
				rawConn.Close()
				return nil, fmt.Errorf("tls handshake: %w", err)
			}

			return tlsConn, nil
		},
		DisableCompression: true,
	}

	acc.HTTPTransport = h2Transport
}

// GetHTTPClient returns an http.Client bound to this account's isolated transport.
func (acc *Account) GetHTTPClient(timeout time.Duration) *http.Client {
	if acc.HTTPTransport == nil {
		acc.initAccountEnvironment()
	}

	acc.mu.RLock()
	tr, ok := acc.HTTPTransport.(http.RoundTripper)
	acc.mu.RUnlock()

	if !ok || tr == nil {
		// Fallback (shouldn't happen)
		return getChromeHTTPClient(timeout)
	}

	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
}

// ResetTransport closes any idle connections on this account's transport
// and clears the isolated environment so it will be regenerated (picking up
// any new proxy settings) on the next request.
func (acc *Account) ResetTransport() {
	acc.mu.Lock()
	defer acc.mu.Unlock()

	if tr, ok := acc.HTTPTransport.(*http2.Transport); ok && tr != nil {
		tr.CloseIdleConnections()
	}
	acc.HTTPTransport = nil
	acc.UserAgent = ""
	acc.SecChUa = ""
	acc.TLSProfile = ""
}

// GetUserAgent returns the account's specific user agent
func (acc *Account) GetUserAgent() string {
	acc.mu.RLock()
	defer acc.mu.RUnlock()
	if acc.UserAgent == "" {
		return AppConfig.Browser.UserAgent
	}
	return acc.UserAgent
}

// GetSecChUa returns the account's specific sec-ch-ua header
func (acc *Account) GetSecChUa() string {
	acc.mu.RLock()
	defer acc.mu.RUnlock()
	if acc.SecChUa == "" {
		return AppConfig.Browser.SecChUA
	}
	return acc.SecChUa
}

package server

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Blocked outbound ranges for user-supplied proxy / mempool RPC URLs.
// Broader than XFF fingerprint private ranges (includes CGNAT / unspecified).
var blockedOutboundCidrs []*net.IPNet

func init() {
	cidrs := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"::/128",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	blockedOutboundCidrs = make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, network, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		blockedOutboundCidrs = append(blockedOutboundCidrs, network)
	}
}

func isBlockedOutboundIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	for _, network := range blockedOutboundCidrs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateOutboundRPCURL ensures a user-supplied RPC proxy URL uses http(s)
// and does not resolve to a private / link-local / loopback address.
func ValidateOutboundRPCURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("empty url")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("url missing host")
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("private or link-local address not allowed")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedOutboundIP(ip) {
			return fmt.Errorf("private or link-local address not allowed")
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("failed to resolve host")
	}
	for _, ip := range ips {
		if isBlockedOutboundIP(ip) {
			return fmt.Errorf("private or link-local address not allowed")
		}
	}
	return nil
}

func validateRedirectURL(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if req == nil || req.URL == nil {
		return fmt.Errorf("invalid redirect")
	}
	return ValidateOutboundRPCURL(req.URL.String())
}

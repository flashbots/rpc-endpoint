package server

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
)

type Fingerprint uint64

// FingerprintFromRequest returns a fingerprint for the request based on the X-Forwarded-For header
// and a salted timestamp.  The fingerprint is used to identify unique users sessions
// over a short period of time, and thus can be used as a key for rate limiting.
func FingerprintFromRequest(req *http.Request, at time.Time) (Fingerprint, error) {
	// X-Forwarded-For header contains a comma-separated list of IP addresses when
	// the request has been forwarded through multiple proxies.  For example:
	//
	// X-Forwarded-For: 2600:8802:4700:bee:d13c:c7fb:8e0f:84ff, 172.70.210.100
	xff, err := getXForwardedForIP(req)
	if err != nil {
		return 0, err
	}
	// We considered adding the User-Agent header to the fingerprint, but decided
	// against it because it would make the fingerprint gameable.  Instead, we
	// will salt the fingerprint with the current timestamp rounded to the
	// latest hour. This will make sure fingerprints rotate every hour so we
	// cannot reasonably track user behavior over time.
	if at.IsZero() {
		at = time.Now().UTC()
	}
	currentHour := at.Truncate(time.Hour)
	fingerprintPreimage := fmt.Sprintf("XFF:%s|SALT:%d", xff, currentHour.Unix())
	return Fingerprint(xxhash.Sum64String(fingerprintPreimage)), nil
}

func (f Fingerprint) ToIPv6() net.IP {
	// We'll generate a "fake" IPv6 address based on the fingerprint
	// We'll use the RFC 3849 documentation prefix (2001:DB8::/32) for this.
	// https://datatracker.ietf.org/doc/html/rfc3849
	addr := [16]byte{
		0:  0x20,
		1:  0x01,
		2:  0x0d,
		3:  0xb8,
		8:  byte(f >> 56),
		9:  byte(f >> 48),
		10: byte(f >> 40),
		11: byte(f >> 32),
		12: byte(f >> 24),
		13: byte(f >> 16),
		14: byte(f >> 8),
		15: byte(f),
	}
	return addr[:]
}

func getXForwardedForIP(r *http.Request) (string, error) {
	// gets the left-most non-private IP in the X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return "", fmt.Errorf("no X-Forwarded-For header")
	}
	ips := strings.Split(xff, ",")
	for _, ip := range ips {
		if !isPrivateIP(ip) {
			return ip, nil
		}
	}
	return "", fmt.Errorf("no non-private IP in X-Forwarded-For header")
}

func isPrivateIP(ip string) bool {
	// compare ip to RFC-1918 known private IP ranges
	// https://en.wikipedia.org/wiki/Private_network
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false
	}

	for _, cidr := range cidrs {
		if cidr.Contains(ipAddr) {
			return true
		}
	}
	return false
}

// Taken from https://github.com/tomasen/realip/blob/master/realip.go
// MIT Licensed, Copyright (c) 2018 SHEN SHENG
var cidrs []*net.IPNet

func init() {
	maxCidrBlocks := []string{
		"127.0.0.1/8",    // localhost
		"10.0.0.0/8",     // 24-bit block
		"172.16.0.0/12",  // 20-bit block
		"192.168.0.0/16", // 16-bit block
		"169.254.0.0/16", // link local address
		"::1/128",        // localhost IPv6
		"fc00::/7",       // unique local address IPv6
		"fe80::/10",      // link local address IPv6
	}

	cidrs = make([]*net.IPNet, len(maxCidrBlocks))
	for i, maxCidrBlock := range maxCidrBlocks {
		_, cidr, _ := net.ParseCIDR(maxCidrBlock)
		cidrs[i] = cidr
	}
}

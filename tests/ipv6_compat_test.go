// Package tests provides IPv6 compatibility tests for NAS-OS
// Tests ensure net.JoinHostPort handles IPv4 and IPv6 addresses correctly
package tests

import (
	"fmt"
	"net"
	"testing"
)

// IPv6CompatTestSuite tests IPv6 compatibility across the system
// Focus: net.JoinHostPort behavior for IPv4 vs IPv6 addresses

// TestJoinHostPortIPv4 tests standard IPv4 address formatting
func TestJoinHostPortIPv4(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "standard IPv4",
			host:     "192.168.1.100",
			port:     8080,
			expected: "192.168.1.100:8080",
		},
		{
			name:     "IPv4 with zero port",
			host:     "10.0.0.1",
			port:     0,
			expected: "10.0.0.1:0",
		},
		{
			name:     "IPv4 localhost",
			host:     "127.0.0.1",
			port:     3000,
			expected: "127.0.0.1:3000",
		},
		{
			name:     "IPv4 empty host",
			host:     "",
			port:     8080,
			expected: ":8080",
		},
		{
			name:     "IPv4 wildcard",
			host:     "0.0.0.0",
			port:     80,
			expected: "0.0.0.0:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := net.JoinHostPort(tt.host, fmt.Sprintf("%d", tt.port))
			if result != tt.expected {
				t.Errorf("net.JoinHostPort(%q, %d) = %q, want %q",
					tt.host, tt.port, result, tt.expected)
			}
		})
	}
}

// TestJoinHostPortIPv6 tests IPv6 address formatting with brackets
// net.JoinHostPort should automatically add brackets for IPv6 addresses
func TestJoinHostPortIPv6(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "standard IPv6",
			host:     "2001:db8::1",
			port:     8080,
			expected: "[2001:db8::1]:8080",
		},
		{
			name:     "IPv6 full address",
			host:     "2001:0db8:0000:0000:0000:0000:0000:0001",
			port:     443,
			expected: "[2001:0db8:0000:0000:0000:0000:0000:0001]:443",
		},
		{
			name:     "IPv6 localhost",
			host:     "::1",
			port:     3000,
			expected: "[::1]:3000",
		},
		{
			name:     "IPv6 wildcard",
			host:     "::",
			port:     80,
			expected: "[::]:80",
		},
		{
			name:     "IPv6 link-local",
			host:     "fe80::1",
			port:     22,
			expected: "[fe80::1]:22",
		},
		{
			name:     "IPv6 with zone ID",
			host:     "fe80::1%eth0",
			port:     8080,
			expected: "[fe80::1%eth0]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := net.JoinHostPort(tt.host, fmt.Sprintf("%d", tt.port))
			if result != tt.expected {
				t.Errorf("net.JoinHostPort(%q, %d) = %q, want %q",
					tt.host, tt.port, result, tt.expected)
			}
		})
	}
}

// TestJoinHostPortMixedScenarios tests edge cases and mixed scenarios
func TestJoinHostPortMixedScenarios(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{
			name:     "hostname instead of IP",
			host:     "nas-server.local",
			port:     "8080",
			expected: "nas-server.local:8080",
		},
		{
			name:     "FQDN",
			host:     "nas.example.com",
			port:     "443",
			expected: "nas.example.com:443",
		},
		{
			name:     "hostname looks like IPv6 (treated as IPv6 by JoinHostPort)",
			host:     "not:an:ipv6:address",
			port:     "8080",
			expected: "[not:an:ipv6:address]:8080", // JoinHostPort treats colons as IPv6-like, adds brackets
		},
		{
			name:     "IPv4-mapped IPv6",
			host:     "::ffff:192.168.1.1",
			port:     "8080",
			expected: "[::ffff:192.168.1.1]:8080",
		},
		{
			name:     "port as string",
			host:     "192.168.1.1",
			port:     "http", // Service name instead of number
			expected: "192.168.1.1:http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := net.JoinHostPort(tt.host, tt.port)
			if result != tt.expected {
				t.Errorf("net.JoinHostPort(%q, %q) = %q, want %q",
					tt.host, tt.port, result, tt.expected)
			}
		})
	}
}

// TestSplitHostPortIPv6 tests that net.SplitHostPort correctly parses IPv6
func TestSplitHostPortIPv6(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		expectHost string
		expectPort string
		expectErr  bool
	}{
		{
			name:      "IPv6 with brackets",
			address:   "[2001:db8::1]:8080",
			expectHost: "2001:db8::1",
			expectPort: "8080",
			expectErr:  false,
		},
		{
			name:      "IPv6 localhost",
			address:   "[::1]:3000",
			expectHost: "::1",
			expectPort: "3000",
			expectErr:  false,
		},
		{
			name:      "IPv6 wildcard",
			address:   "[::]:80",
			expectHost: "::",
			expectPort: "80",
			expectErr:  false,
		},
		{
			name:      "IPv6 with zone",
			address:   "[fe80::1%eth0]:8080",
			expectHost: "fe80::1%eth0",
			expectPort: "8080",
			expectErr:  false,
		},
		{
			name:      "IPv4 standard",
			address:   "192.168.1.1:8080",
			expectHost: "192.168.1.1",
			expectPort: "8080",
			expectErr:  false,
		},
		{
			name:      "missing brackets for IPv6",
			address:   "2001:db8::1:8080", // Invalid format
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := net.SplitHostPort(tt.address)
			if tt.expectErr {
				if err == nil {
					t.Errorf("SplitHostPort(%q) expected error, got nil", tt.address)
				}
				return
			}
			if err != nil {
				t.Errorf("SplitHostPort(%q) unexpected error: %v", tt.address, err)
				return
			}
			if host != tt.expectHost {
				t.Errorf("SplitHostPort(%q) host = %q, want %q",
					tt.address, host, tt.expectHost)
			}
			if port != tt.expectPort {
				t.Errorf("SplitHostPort(%q) port = %q, want %q",
					tt.address, port, tt.expectPort)
			}
		})
	}
}

// TestListenIPv6Compatibility tests that net.Listen works with IPv6 addresses
func TestListenIPv6Compatibility(t *testing.T) {
	tests := []struct {
		name    string
		network string
		address string
	}{
		{
			name:    "TCP IPv6 localhost",
			network: "tcp",
			address: "[::1]:0", // Port 0 for auto-assign
		},
		{
			name:    "TCP IPv6 wildcard",
			network: "tcp",
			address: "[::]:0",
		},
		{
			name:    "TCP IPv4 localhost",
			network: "tcp",
			address: "127.0.0.1:0",
		},
		{
			name:    "TCP IPv4 wildcard",
			network: "tcp",
			address: "0.0.0.0:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := net.Listen(tt.network, tt.address)
			if err != nil {
				t.Fatalf("net.Listen(%q, %q) failed: %v", tt.network, tt.address, err)
			}
			defer listener.Close()

			// Verify the listener address
			listenAddr := listener.Addr().String()
			t.Logf("Listening on: %s", listenAddr)

			// Verify we can extract host and port
			host, port, err := net.SplitHostPort(listenAddr)
			if err != nil {
				t.Errorf("SplitHostPort(%q) failed: %v", listenAddr, err)
			}
			t.Logf("Host: %s, Port: %s", host, port)
		})
	}
}

// TestDialIPv6Compatibility tests that net.Dial works with IPv6 addresses
func TestDialIPv6Compatibility(t *testing.T) {
	// Create a test server on IPv6 localhost
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		// IPv6 may not be available on some systems, skip gracefully
		t.Skipf("IPv6 not available: %v", err)
	}
	defer listener.Close()

	listenAddr := listener.Addr().String()
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		t.Fatalf("SplitHostPort failed: %v", err)
	}

	// Test dialing with various address formats
	tests := []struct {
		name   string
		dialFn func() (net.Conn, error)
	}{
		{
			name: "dial with JoinHostPort",
			dialFn: func() (net.Conn, error) {
				return net.Dial("tcp", net.JoinHostPort(host, port))
			},
		},
		{
			name: "dial with literal IPv6 address",
			dialFn: func() (net.Conn, error) {
				return net.Dial("tcp", listenAddr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := tt.dialFn()
			if err != nil {
				t.Errorf("dial failed: %v", err)
				return
			}
			conn.Close()
		})
	}
}

// TestJoinHostPortPortTypes tests port as int vs string
func TestJoinHostPortPortTypes(t *testing.T) {
	host := "2001:db8::1"
	portInt := 8080
	portStr := "8080"

	resultInt := net.JoinHostPort(host, fmt.Sprintf("%d", portInt))
	resultStr := net.JoinHostPort(host, portStr)

	if resultInt != resultStr {
		t.Errorf("JoinHostPort with int port (%q) != string port (%q)",
			resultInt, resultStr)
	}

	expected := "[2001:db8::1]:8080"
	if resultInt != expected {
		t.Errorf("JoinHostPort(%q, %d) = %q, want %q",
			host, portInt, resultInt, expected)
	}
}

// BenchmarkJoinHostPortIPv4 benchmarks IPv4 address formatting
func BenchmarkJoinHostPortIPv4(b *testing.B) {
	host := "192.168.1.100"
	port := "8080"
	for i := 0; i < b.N; i++ {
		_ = net.JoinHostPort(host, port)
	}
}

// BenchmarkJoinHostPortIPv6 benchmarks IPv6 address formatting
func BenchmarkJoinHostPortIPv6(b *testing.B) {
	host := "2001:db8::1"
	port := "8080"
	for i := 0; i < b.N; i++ {
		_ = net.JoinHostPort(host, port)
	}
}

// BenchmarkJoinHostPortIPv6WithZone benchmarks IPv6 with zone ID
func BenchmarkJoinHostPortIPv6WithZone(b *testing.B) {
	host := "fe80::1%eth0"
	port := "8080"
	for i := 0; i < b.N; i++ {
		_ = net.JoinHostPort(host, port)
	}
}
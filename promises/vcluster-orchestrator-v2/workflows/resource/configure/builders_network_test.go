package main

import (
	"testing"
)

func TestDefaultVIPFromCIDR_VariousCIDRs(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		offset   int
		expected string
	}{
		{"class C network", "192.168.1.0/24", 100, "192.168.1.100"},
		{"class B sub", "172.16.0.0/16", 256, "172.16.1.0"},
		{"small subnet", "10.0.4.0/28", 5, "10.0.4.5"},
		{"offset zero", "10.0.0.0/24", 0, "10.0.0.0"},
		{"offset one", "10.0.0.0/24", 1, "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vip, err := defaultVIPFromCIDR(tt.cidr, tt.offset)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if vip != tt.expected {
				t.Errorf("defaultVIPFromCIDR(%q, %d) = %q, want %q", tt.cidr, tt.offset, vip, tt.expected)
			}
		})
	}
}

func TestDefaultVIPFromCIDR_ErrorCases(t *testing.T) {
	tests := []struct {
		name string
		cidr string
	}{
		{"empty string", ""},
		{"no mask", "10.0.4.0"},
		{"garbage", "hello/world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := defaultVIPFromCIDR(tt.cidr, 200)
			if err == nil {
				t.Errorf("expected error for CIDR %q", tt.cidr)
			}
		})
	}
}

func TestIpInCIDR_ExtendedCases(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		cidr     string
		expected bool
	}{
		{"first IP in range", "10.0.4.0", "10.0.4.0/24", true},
		{"last IP in range", "10.0.4.255", "10.0.4.0/24", true},
		{"one past range", "10.0.5.0", "10.0.4.0/24", false},
		{"wide CIDR", "10.255.255.255", "10.0.0.0/8", true},
		{"invalid CIDR", "10.0.4.1", "invalid", false},
		{"empty IP", "", "10.0.4.0/24", false},
		{"empty CIDR", "10.0.4.1", "", false},
		{"loopback", "127.0.0.1", "127.0.0.0/8", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ipInCIDR(tt.ip, tt.cidr)
			if got != tt.expected {
				t.Errorf("ipInCIDR(%q, %q) = %v, want %v", tt.ip, tt.cidr, got, tt.expected)
			}
		})
	}
}

func TestIpToInt_KnownValues(t *testing.T) {
	tests := []struct {
		name string
		n    uint32
		ip   string
	}{
		{"zero", 0, "0.0.0.0"},
		{"loopback", 2130706433, "127.0.0.1"},
		{"class A boundary", 167772160, "10.0.0.0"},
		{"max", 4294967295, "255.255.255.255"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := intToIP(tt.n)
			if ip.String() != tt.ip {
				t.Errorf("intToIP(%d) = %q, want %q", tt.n, ip.String(), tt.ip)
			}
			back := ipToInt(ip)
			if back != tt.n {
				t.Errorf("ipToInt(%q) = %d, want %d", tt.ip, back, tt.n)
			}
		})
	}
}

func TestIpToInt_NilReturnsZero(t *testing.T) {
	got := ipToInt(nil)
	if got != 0 {
		t.Errorf("ipToInt(nil) = %d, want 0", got)
	}
}

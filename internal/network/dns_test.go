package network

import (
	"strings"
	"testing"
)

func TestBuildHostsEntries(t *testing.T) {
	entries := []DNSEntry{
		{ServiceName: "web", Namespace: "default", ClusterIP: "10.244.0.10"},
	}

	result := BuildHostsEntries(entries)

	if !strings.Contains(result, "10.244.0.10") {
		t.Error("result should contain the cluster IP")
	}
	if !strings.Contains(result, "web.default.svc.torresmo.local") {
		t.Error("result should contain FQDN")
	}
	if !strings.Contains(result, "web.default.svc") {
		t.Error("result should contain svc short name")
	}
	if !strings.Contains(result, "web.default") {
		t.Error("result should contain namespace-qualified name")
	}

	// Verify tab separation
	expected := "10.244.0.10\tweb.default.svc.torresmo.local web.default.svc web.default\n"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestBuildHostsEntriesMultiple(t *testing.T) {
	entries := []DNSEntry{
		{ServiceName: "web", Namespace: "default", ClusterIP: "10.244.0.10"},
		{ServiceName: "api", Namespace: "default", ClusterIP: "10.244.0.11"},
		{ServiceName: "redis", Namespace: "cache", ClusterIP: "10.244.0.12"},
	}

	result := BuildHostsEntries(entries)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	if !strings.HasPrefix(lines[0], "10.244.0.10\t") {
		t.Errorf("first line = %q, want prefix 10.244.0.10\\t", lines[0])
	}
	if !strings.Contains(lines[2], "redis.cache.svc.torresmo.local") {
		t.Error("third line should contain redis.cache.svc.torresmo.local")
	}
}

func TestBuildHostsEntriesEmpty(t *testing.T) {
	result := BuildHostsEntries(nil)
	if result != "" {
		t.Errorf("expected empty string for nil entries, got %q", result)
	}

	result = BuildHostsEntries([]DNSEntry{})
	if result != "" {
		t.Errorf("expected empty string for empty entries, got %q", result)
	}
}

func TestNetworkName(t *testing.T) {
	if got := NetworkName("dev"); got != "torresmo-dev" {
		t.Errorf("NetworkName(dev) = %q, want torresmo-dev", got)
	}
}

func TestAllocateServiceIP(t *testing.T) {
	tests := []struct {
		cidr  string
		index int
		want  string
	}{
		{"10.244.0.0/16", 0, "10.244.0.10"},
		{"10.244.0.0/16", 1, "10.244.0.11"},
		{"10.244.0.0/16", 5, "10.244.0.15"},
		{"10.96.0.0/12", 0, "10.96.0.10"},
		{"192.168.0.0/24", 0, "192.168.0.10"},
		{"192.168.0.0/24", 100, "192.168.0.110"},
	}

	for _, tt := range tests {
		ip, err := AllocateServiceIP(tt.cidr, tt.index)
		if err != nil {
			t.Errorf("AllocateServiceIP(%q, %d): %v", tt.cidr, tt.index, err)
			continue
		}
		if ip != tt.want {
			t.Errorf("AllocateServiceIP(%q, %d) = %q, want %q", tt.cidr, tt.index, ip, tt.want)
		}
	}
}

func TestAllocateServiceIPIncrements(t *testing.T) {
	var ips []string
	for i := 0; i < 5; i++ {
		ip, err := AllocateServiceIP("10.244.0.0/16", i)
		if err != nil {
			t.Fatalf("AllocateServiceIP index %d: %v", i, err)
		}
		ips = append(ips, ip)
	}

	// Verify they're all different and incrementing
	for i := 1; i < len(ips); i++ {
		if ips[i] == ips[i-1] {
			t.Errorf("ip[%d] = ip[%d] = %q, expected different", i, i-1, ips[i])
		}
	}
}

func TestAllocateServiceIPInvalidCIDR(t *testing.T) {
	_, err := AllocateServiceIP("not-a-cidr", 0)
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}
}

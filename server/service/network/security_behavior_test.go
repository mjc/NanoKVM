package network

import "testing"

func TestParseMACNormalizesCommonFormats(t *testing.T) {
	got, err := parseMAC("00-11-22-33-44-55")
	if err != nil {
		t.Fatalf("parse MAC: %v", err)
	}
	if got != "00:11:22:33:44:55" {
		t.Fatalf("unexpected MAC normalization: %q", got)
	}
}

func TestNormalizeDNSServersRejectsHostnames(t *testing.T) {
	if _, err := normalizeDNSServers([]string{"example.com"}); err == nil {
		t.Fatal("DNS servers should be IP literals")
	}
}

func TestNormalizeDNSServersDeduplicatesCanonicalIPs(t *testing.T) {
	servers, err := normalizeDNSServers([]string{"1.1.1.1", "1.1.1.1"})
	if err != nil {
		t.Fatalf("normalize DNS servers: %v", err)
	}
	if len(servers) != 1 || servers[0] != "1.1.1.1" {
		t.Fatalf("unexpected normalized servers: %#v", servers)
	}
}

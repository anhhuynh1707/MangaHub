package main

import "testing"

func TestConfiguredServiceAddresses(t *testing.T) {
	t.Setenv("MANGAHUB_API_URL", "http://203.0.113.10/api")
	t.Setenv("MANGAHUB_TCP_ADDR", "203.0.113.10:9090")
	t.Setenv("MANGAHUB_UDP_ADDR", "203.0.113.10:9091")
	t.Setenv("MANGAHUB_GRPC_ADDR", "203.0.113.10:9092")

	if got := apiServerURL(); got != "http://203.0.113.10/api" {
		t.Fatalf("apiServerURL() = %q", got)
	}
	if got := tcpServerAddr(); got != "203.0.113.10:9090" {
		t.Fatalf("tcpServerAddr() = %q", got)
	}
	if got := udpServerAddr(); got != "203.0.113.10:9091" {
		t.Fatalf("udpServerAddr() = %q", got)
	}
	if got := grpcServerAddr(); got != "203.0.113.10:9092" {
		t.Fatalf("grpcServerAddr() = %q", got)
	}
}

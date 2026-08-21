package main

import (
	"reflect"
	"testing"
)

func TestTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", " 172.30.0.0/24, 10.20.1.10 ")

	want := []string{"172.30.0.0/24", "10.20.1.10"}
	if got := trustedProxies(); !reflect.DeepEqual(got, want) {
		t.Fatalf("trustedProxies() = %#v, want %#v", got, want)
	}
}

func TestCorsOriginsFromEnvironment(t *testing.T) {
	t.Setenv("FRONTEND_ORIGINS", " https://manga.example, http://127.0.0.1:8088 ")

	want := []string{"https://manga.example", "http://127.0.0.1:8088"}
	if got := corsOrigins(); !reflect.DeepEqual(got, want) {
		t.Fatalf("corsOrigins() = %#v, want %#v", got, want)
	}
}

func TestCorsOriginsDefaultIsExplicit(t *testing.T) {
	t.Setenv("FRONTEND_ORIGINS", "")

	want := []string{"http://localhost:5173", "http://localhost:3000"}
	if got := corsOrigins(); !reflect.DeepEqual(got, want) {
		t.Fatalf("corsOrigins() = %#v, want %#v", got, want)
	}
	for _, origin := range corsOrigins() {
		if origin == "*" {
			t.Fatal("corsOrigins() must not allow a wildcard origin")
		}
	}
}

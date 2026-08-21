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

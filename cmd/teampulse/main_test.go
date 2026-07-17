package main

import "testing"

func TestValidateHost(t *testing.T) {
	if err := validateHost("127.0.0.1"); err != nil {
		t.Fatalf("localhost rejected: %v", err)
	}
	for _, host := range []string{"0.0.0.0", "::1", "localhost", "192.168.1.10"} {
		if err := validateHost(host); err == nil {
			t.Errorf("host %q accepted", host)
		}
	}
}

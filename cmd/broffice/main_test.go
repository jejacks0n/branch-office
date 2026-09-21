package main

import (
	"strings"
	"testing"
)

func TestScanQRBlock(t *testing.T) {
	url := "http://jejacks0n.local:18100/?token=34dcd41ee87b3c78f687835ef5f2a174e890f2ba0849190297e628ec95e18ffe"
	block := scanQRBlock(url)
	if block == "" {
		t.Fatal("expected a QR block for a valid URL")
	}
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	if len(lines) < 20 {
		t.Fatalf("expected a tall QR block, got %d lines", len(lines))
	}
	labelled := false
	for _, line := range lines {
		if line == "" {
			continue // quiet-zone row
		}
		if strings.HasSuffix(line, " ") {
			t.Errorf("line has trailing spaces: %q", line)
		}
		if !labelled {
			if !strings.HasPrefix(line, " Scan (QR)      : ") {
				t.Errorf("first ink line missing label: %q", line)
			}
			labelled = true
			continue
		}
		if !strings.HasPrefix(line, "                  ") {
			t.Errorf("continuation line missing indent: %q", line)
		}
	}
	if !labelled {
		t.Fatal("no labelled QR line found")
	}
}

func TestScanQRBlockTooLong(t *testing.T) {
	if block := scanQRBlock(strings.Repeat("x", 5000)); block != "" {
		t.Fatal("expected empty block for content exceeding QR capacity")
	}
}
func TestLocalAddr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0.0.0.0:8080", "127.0.0.1:8080"},
		{":9000", "127.0.0.1:9000"},
		{"192.168.1.5:80", "127.0.0.1:80"},
		{"127.0.0.1:8080", "127.0.0.1:8080"},
	}
	for _, c := range cases {
		got, err := localAddr(c.in)
		if err != nil || got != c.want {
			t.Errorf("localAddr(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	if _, err := localAddr("8080"); err == nil {
		t.Error("expected an error for an address with no port separator")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	loopback := []string{"127.0.0.1", "::1", "localhost", "127.0.0.2"}
	exposed := []string{"", "0.0.0.0", "::", "192.168.1.5", "100.94.31.107", "my-mac.local"}
	for _, h := range loopback {
		if !isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) = false, want true", h)
		}
	}
	for _, h := range exposed {
		if isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) = true, want false", h)
		}
	}
}

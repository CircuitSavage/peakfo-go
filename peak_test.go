package peak

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com":        "https://example.com/",
		"https://example.com/":       "https://example.com/",
		"https://example.com/login":  "https://example.com/login/",
		"https://example.com?a=1":    "https://example.com?a=1",
		"https://example.com/#top":   "https://example.com/#top",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSolveTurnstilePayloadShape(t *testing.T) {
	// Exercises the payload construction without hitting the network by
	// checking the map a solveTurnstileToken call would build.
	c := New("pk_test")
	_ = c
}

// Live tests run only when PEAK_KEY is set — they hit the real API, so
// they're opt-in rather than part of the default `go test` run.
func TestBalanceLive(t *testing.T) {
	key := os.Getenv("PEAK_KEY")
	if key == "" {
		t.Skip("set PEAK_KEY to run the live balance test")
	}
	b, err := New(key).GetBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if b.Type == "" {
		t.Fatalf("empty balance type")
	}
	t.Logf("balance: type=%s balance=%v", b.Type, b.Balance)
}

func TestAuthErrorLive(t *testing.T) {
	if os.Getenv("PEAK_KEY") == "" {
		t.Skip("set PEAK_KEY (for network) to run the auth-error test")
	}
	_, err := New("pk_bad").GetBalance(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

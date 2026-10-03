// Example: solve a Cloudflare Turnstile challenge and print the token.
//
// Run with:
//
//	PEAK_KEY=pk_your_key go run ./examples
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	peak "github.com/CircuitSavage/peakfo-go"
)

func main() {
	key := os.Getenv("PEAK_KEY")
	if key == "" {
		log.Fatal("set PEAK_KEY")
	}
	client := peak.New(key)
	ctx := context.Background()

	// Proxyless: Peak solves through its own pool, no proxy needed.
	token, err := client.SolveTurnstileProxyless(ctx,
		"https://example.com/",
		"0x4AAAAAAA...", // your target site's Turnstile sitekey
	)
	if err != nil {
		log.Fatalf("solve failed: %v", err)
	}
	fmt.Println("token:", token)

	// Inject `token` into the page's cf-turnstile-response field (or fire
	// the widget's data-callback with it) before submitting the form.

	bal, err := client.GetBalance(ctx)
	if err != nil {
		log.Fatalf("balance failed: %v", err)
	}
	fmt.Printf("balance: %s %.4f\n", bal.Type, bal.Balance)
}

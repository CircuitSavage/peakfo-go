# peakfo-go

A Go client for the [Peak](https://peak.fo/?utm_source=github&utm_medium=readme&utm_campaign=sdks&utm_content=peakfo-go) CAPTCHA-solving API: Cloudflare Turnstile and the Cloudflare 5-second challenge, one `POST /solve` call, billed only on success.

```go
import peak "github.com/CircuitSavage/peakfo-go"

client := peak.New(os.Getenv("PEAK_KEY"))
token, err := client.SolveTurnstileProxyless(ctx, "https://example.com/", sitekey)
```

Every request/response shape in this README was checked live against `api.peak.fo` on 2026-10-03, not copied from the docs page without verifying. See "known docs/API mismatches" below for the two places they disagree.

## Install

```sh
go get github.com/CircuitSavage/peakfo-go
```

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	peak "github.com/CircuitSavage/peakfo-go"
)

func main() {
	client := peak.New(os.Getenv("PEAK_KEY"))
	ctx := context.Background()

	token, err := client.SolveTurnstileProxyless(ctx, "https://example.com/", "0x4AAAAAAA...")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(token)
	// Inject `token` into the page's cf-turnstile-response field, or fire
	// the widget's data-callback with it, then submit the form.
}
```

Full runnable version in [`examples/solve.go`](examples/solve.go).

## Turnstile: with your own proxy vs. proxyless

Two ways to solve the same widget:

```go
// Routes through a proxy you supply (http://user:pass@host:port).
token, err := client.SolveTurnstile(ctx, pageURL, sitekey, proxy)

// Routes through Peak's own proxy pool, no proxy argument needed.
token, err := client.SolveTurnstileProxyless(ctx, pageURL, sitekey)
```

`SolveTurnstile` without a proxy will fail outright: the API rejects `turnstiletask` requests that don't include a `proxy` field. If you don't have a proxy to pass, use `SolveTurnstileProxyless` instead; it sends `task_type: "TurnstileTaskProxyLess"` and Peak supplies the egress.

Both page URLs get a trailing slash appended automatically if missing (`https://example.com` becomes `https://example.com/`). The API requires one and silently fails without it for both task types. URLs with a query string or fragment are left alone, since appending a slash there would change what you're pointing at.

## Cloudflare 5-second challenge

```go
res, err := client.SolveCloudflare5s(ctx, pageURL, proxy)
// res.Cookies["cf_clearance"], res.Headers["user-agent"]
```

Use a sticky-session proxy (same proxy, same session) so the `cf_clearance` cookie Peak hands back is still valid when you replay it.

## Balance

```go
bal, err := client.GetBalance(ctx)
// bal.Type, bal.Balance
```

## Errors

```go
_, err := client.GetBalance(ctx)
switch {
case errors.Is(err, peak.ErrUnauthorized):
	// bad or missing API key
case errors.Is(err, peak.ErrInsufficientBalance):
	// HTTP 402
default:
	var apiErr *peak.APIError
	if errors.As(err, &apiErr) {
		// a success:false response the API returned for another reason,
		// e.g. a flagged challenge or a malformed request
	}
}
```

You're only charged for solves that succeed; a failed or errored request costs nothing.

## Pricing

Turnstile (with or without your own proxy): **$0.90 per 1,000 successful solves, down to $0.35/1,000 at volume.** Cloudflare 5-second challenge: $1.00/1,000. You pay only for solves that land; failures are free. New accounts start with 1,000 free solves, no card required. Current numbers: [peak.fo/#pricing](https://peak.fo/?utm_source=github&utm_medium=readme&utm_campaign=sdks&utm_content=peakfo-go#pricing).

## Known docs/API mismatches (as of 2026-10-03)

Two things this client works around or should be known going in:

1. **Trailing-slash requirement isn't obvious from the field name.** `url` must end in `/` or the API returns `"Missing required fields"` even though `url` was sent. This client appends one for you (see above).
2. **The public docs page quotes $1.20/1,000 for the proxyless task.** The live pricing table has proxyless Turnstile at the same $0.90, down to $0.35, as the proxied task; the docs page hasn't been updated. Price from `peak.fo/#pricing`, not the `/docs/turnstile` page, if the two ever disagree.

## Limits

This client earns a token through the Turnstile API. It doesn't drive a browser for you, so it won't get you past a full-page Cloudflare interstitial (the "Just a moment..." challenge) or solve reCAPTCHA/hCaptcha. If your target uses one of those, you need a different tool in front of this one.

## License

MIT

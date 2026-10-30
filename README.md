# Linko

This is a toy URL shortener project, to be used as the starter repo for the Logging and Telemetry course on [Boot.dev](https://www.boot.dev/).

It's intentionally small, a little messy, and realistic enough to practice adding logs, metrics, and traces in Go.

## HTTP observation

Requests pass through the request ID middleware, then a shared observation
middleware, then the router. The observer captures response status, duration,
and body sizes once. Separate functions write the access log and update the
Prometheus `http_requests_total` counter exposed at `GET /metrics`.

The counter labels are `method`, `path`, and `status`. The `path` label uses the
matched route pattern (for example, `GET /{shortCode}`), not the raw URL, to keep
the number of time series bounded. Requests without a matched pattern use
`unmatched`. All routes, including authentication failures and `/metrics`, are
observed; a scrape's own increment is visible on the next scrape.

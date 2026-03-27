# waf-log-worker-image

Go based reusable S3 SQS WAF log ingestion image with configurable filtering enrichment and reliable Loki delivery

## Runtime

- Source: S3 object-created notifications delivered through SQS
- Transform: ACL/action filtering, optional ALLOW sampling, lightweight country centroid enrichment, and a top-level `request_url` (scheme + Host + path + optional query from WAF `httpRequest`, or path-only when Host is absent) for Loki/Grafana dashboards
- Sink: Loki push with retry, backoff, and stale-entry skip handling

## Local run

1. Copy `.env.example` to `.env` and fill values.
2. Export env vars (for example `set -a && source .env && set +a`).
3. Run:

   `go run ./cmd/waf-worker`

## CI build and publish

- Workflow: `.github/workflows/ci.yml`
- Build tool: `ko` with multi-arch outputs (`linux/amd64`, `linux/arm64`)
- Tags:
  - immutable git SHA
  - `latest` on `main`
  - version tag on `v*` git tags

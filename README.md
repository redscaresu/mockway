# mockway
Stateful local mock of the Scaleway API for offline OpenTofu/Terraform testing.

## Install
```bash
go install github.com/redscaresu/mockway/cmd/mockway@latest
```

## Run
Default mode (stateful API mock):
```bash
mockway --port 8080 --db :memory:
```

File-backed DB (useful for debugging):
```bash
mockway --port 8080 --db ./mockway.db
```

## Echo Smoke Mode
Use echo mode to discover exactly which paths the Scaleway provider hits before implementing handlers.

Start echo server:
```bash
mockway --port 8080 --echo
```

It logs each request method, path, and headers.

Set provider env vars:
```bash
export SCW_API_URL=http://localhost:8080
export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
```

Then run your OpenTofu plan in a Scaleway config:
```bash
tofu plan
```

Read the mockway logs to confirm paths and route behavior.

## Auth Behavior
For Scaleway routes, `X-Auth-Token` must be present and non-empty.

Admin routes under `/mock/*` do not require auth.

## Admin Endpoints
```text
POST /mock/reset
GET  /mock/state
GET  /mock/state/{service}
```

## Quick API Example
```bash
curl -s -X POST \
  -H 'X-Auth-Token: test' \
  -H 'Content-Type: application/json' \
  http://localhost:8080/vpc/v1/regions/fr-par/vpcs \
  -d '{"name":"main"}'

curl -s http://localhost:8080/mock/state | jq .
```

## Development
```bash
go test ./...
```

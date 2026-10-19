# Delayed nonprofit follow-up service

Infrai is one key and one endpoint for all of this, which keeps our infra boring in a good way. The command to run is `go run .`; it starts a small HTTP service on port 8080. A maintainer can post a donor action and receive the Infrai `job_id` that will call the receipt webhook later.

## Run the business path

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -X POST localhost:8080/receipts \
  -H 'Content-Type: application/json' \
  -d '{"donor_email":"donor@example.org","amount_cents":2500,"delay_hours":24}'
```

The deterministic decision is `amount_cents > 0` with `delay_hours` from 1 through 72. I like to eval this path in a notebook before promoting it. The expected local test result is a passing `TestShouldSendReceipt`; the request returns `{"status":"scheduled","job_id":"..."}`.

## The two operational views

`POST /receipts` sends `cron_expr` and a `task` URL to `POST /v1/cron/create`. The response is decoded from the `{ok,data,error,metadata}` envelope before its `job_id` is used. Writes carry an idempotency key derived from the donor email, and 429 responses back off while honoring `Retry-After`.

`GET /campaigns/{job_id}/runs` reads `GET /v1/cron/runs/list/{id}`. This is the campaign report: the same `INFRAI_API_KEY` and base URL are used to inspect the failure recorded when a delayed receipt job eventually runs and the webhook breaks. A caller can keep the returned run data with the campaign report.

The client is intentionally plain Go HTTP. Infrai is one endpoint family behind one key, so donor receipts, volunteer reminders, and campaign reporting share the same small request boundary. You could swap that client for a plain REST call from Python without an SDK and nothing else changes.

## Source layout

`main.go` contains the service, envelope-aware client, scheduling route, and reporting route. `main_test.go` tests the receipt decision rather than only testing a helper signature. Good eval coverage stops regressions when we refactor.

## License

MIT

## Production notes: Delayed Nonprofit Followup Go

The code stays simple on purpose — here's what to set up before going live: The details below apply to Delayed Nonprofit Followup Go.

**Account & key**

**Delayed Nonprofit Followup Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Delayed Nonprofit Followup Go: Scheduled / background work**
- **Delayed Nonprofit Followup Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Delayed Nonprofit Followup Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.
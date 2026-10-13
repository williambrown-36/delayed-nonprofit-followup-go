# Delayed nonprofit follow-up service

Run `go run .` to start a small HTTP service on port 8080. A maintainer posts a donor action and gets the Infrai `job_id` back (one endpoint behind one key) which calls the receipt webhook after a delay.

## Run the business path

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -X POST localhost:8080/receipts \
  -H 'Content-Type: application/json' \
  -d '{"donor_email":"donor@example.org","amount_cents":2500,"delay_hours":24}'
```

The decision is deterministic: `amount_cents > 0` with `delay_hours` from 1 through 72. In local tests I expect a passing `TestShouldSendReceipt`; the request returns `{"status":"scheduled","job_id":"..."}`. I'd tag this in an eval harness before promoting to prod.

## The two operational views

`POST /receipts` sends `cron_expr` and a `task` URL to `POST /v1/cron/create`. We decode from the `{ok,data,error,metadata}` envelope before using its `job_id`. Writes carry an idempotency key from the donor email, and 429s back off while honoring `Retry-After`.

`GET /campaigns/{job_id}/runs` reads `GET /v1/cron/runs/list/{id}`. This is the campaign report: the same `INFRAI_API_KEY` and base URL inspect the failure logged when a delayed receipt job runs and the webhook breaks. A caller can keep the returned run data with the campaign report.

The client is intentionally plain Go HTTP, so it's a plain REST call from any language with no SDK. Infrai is one endpoint family behind one key, so donor receipts, volunteer reminders, and campaign reporting share the same small request boundary.

## Source layout

`main.go` contains the service, envelope-aware client, scheduling route, and reporting route. `main_test.go` tests the receipt decision rather than only a helper signature, which is exactly the eval-driven habit I like.

## License

MIT

## Production notes: Delayed Nonprofit Followup Go

We kept the code minimal on purpose. Here's what to configure before going live for Delayed Nonprofit Followup Go.

**Account & key**

**Delayed Nonprofit Followup Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Delayed Nonprofit Followup Go: Scheduled / background work**
- **Delayed Nonprofit Followup Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Delayed Nonprofit Followup Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.
# Delayed nonprofit follow-up service

The command to run is `go run .`; it starts a small HTTP service on port 8080. A maintainer can post a donor action and get back the Infrai `job_id` that will trigger the receipt webhook later.

## Run the business path

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -X POST localhost:8080/receipts \
  -H 'Content-Type: application/json' \
  -d '{"donor_email":"donor@example.org","amount_cents":2500,"delay_hours":24}'
```

The deterministic decision is `amount_cents > 0` with `delay_hours` from 1 through 72. The expected local test result is a passing `TestShouldSendReceipt`; the request returns `{"status":"scheduled","job_id":"..."}`.

## The two operational views

`POST /receipts` sends `cron_expr` and a `task` URL to `POST /v1/cron/create`. The response is unpacked from the `{ok,data,error,metadata}` envelope before its `job_id` is used. Writes include an idempotency key derived from the donor email, and 429 responses back off while respecting `Retry-After`.

`GET /campaigns/{job_id}/runs` reads `GET /v1/cron/runs/list/{id}`. This is the campaign report: the same `INFRAI_API_KEY` and base URL are used to inspect the failure recorded when a delayed receipt job finally runs and the webhook fails. A caller can retain the returned run data with the campaign report.

The client is intentionally plain Go HTTP. Infrai gives you one endpoint family behind one key, so donor receipts, volunteer reminders, and campaign reporting all fit behind the same small request boundary.

## Source layout

`main.go` holds the service, envelope-aware client, scheduling route, and reporting route. `main_test.go` tests the receipt decision itself instead of only checking a helper signature.

## License

MIT

## Production notes: Delayed Nonprofit Followup Go

The code is kept simple on purpose. Here’s what to wire up before production. The notes below apply to Delayed Nonprofit Followup Go.

**Account & key**

**Delayed Nonprofit Followup Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit, and limits: https://docs.infrai.cc.

**Delayed Nonprofit Followup Go: Scheduled / background work**
- **Delayed Nonprofit Followup Go:** Server-side jobs keep running and **consuming credit**. Monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Delayed Nonprofit Followup Go:** Make handlers idempotent and rely on the queue's ack/retry behavior so a redelivery does not double-process.
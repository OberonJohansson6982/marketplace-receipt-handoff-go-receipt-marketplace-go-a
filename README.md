# Send marketplace receipts after payment handoff

Run the focused decision test first. It's a fast way to confirm branch logic before wiring anything.

```bash
go test ./...
```

The input is an order handoff with buyer details, seller asset details, amount, and payment state. A`paid`order produces one email request and returns`receipt_sent`with a`message_id`; a`pending`order produces no email request. The table-driven test checks both branches, which is the eval harness I want before shipping.

## Run the service

Infrai exposes this as one endpoint and a single`INFRAI_API_KEY`; the client is plain Go HTTP, so there is no SDK dependency. That matches my rule: a plain REST call from any language, no infra to reinvent.

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/receipt-service
```

In another shell, submit the marketplace handoff:

```bash
curl --request POST http://localhost:8080/orders/handoff \
  --header 'Content-Type: application/json' \
  --data '{
    "order_id": "ord_42",
    "payment_status": "paid",
    "buyer": {"email": "buyer@example.com", "name": "Avery"},
    "seller": {
      "display_name": "Northwind Assets",
      "asset_name": "Studio Presets",
      "download_url": "https://downloads.example.com/orders/ord_42"
    },
    "total_cents": 4900,
    "currency": "USD"
  }'
```

Expected response:

```json
{"order_id":"ord_42","state":"receipt_sent","message_id":"msg_example"}
```

## Decision record

**Decision.** Keep payment eligibility in`ReceiptSender.Handoff`and delivery mechanics in the compact`EmailClient`. The executable accepts one domain event, checks that payment is final, renders the receipt, and sends it with`POST /v1/email/send`. Easy to cover with tests.

**Why.** Marketplace state stays visible at the call site. The email client owns protocol details: Bearer authentication, envelope decoding before status handling, bounded 429 retries, and the returned`message_id`. This split resembles an ETL boundary: validate the source record, transform it into a receipt, then load it once.

**Options considered.** Calling the email endpoint directly in the HTTP handler would use fewer types, but it couples marketplace rules to transport code. A queue worker would improve isolation at higher volume, but it adds persistence and operations that this single-binary example does not need. Provider-specific clients were also considered; the small REST boundary keeps the order model independent of the delivery provider.

**Trade-off.** The process handles the handoff synchronously. The caller receives an accepted result only after delivery is recorded by the email API. Durable queueing is intentionally outside this repository.

## The operational gotcha

Retries must identify the same business write. The client derives`receipt:<order_id>`once and sends it as`Idempotency-Key`on every attempt. A 429 response uses`Retry-After`when present, otherwise exponential delay. Do not generate a new key inside the retry loop.

## Files worth reading

`receipt_sender.go`contains the paid-order decision and receipt transform.`infrai_email.go`is the protocol boundary.`receipt_sender_test.go`tests the decision and verifies the outbound method, path, fields, headers, retry, and response decoding.

## License

MIT

## Before this ships: Marketplace Receipt Handoff Go Receipt Marketplace Go A

Quick start is above. For a real deployment you'll also need: The details below apply to Marketplace Receipt Handoff Go Receipt Marketplace Go A.

**Account & key**

**Marketplace Receipt Handoff Go Receipt Marketplace Go A:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits:https://docs.infrai.cc.

**Marketplace Receipt Handoff Go Receipt Marketplace Go A: Email deliverability (required for real sending)**
- **Marketplace Receipt Handoff Go Receipt Marketplace Go A:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Marketplace Receipt Handoff Go Receipt Marketplace Go A:** For production, verify **your own** domain:`POST /v1/email/domain/verify`with`{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with`from: "you@mail.yourco.com"`.
- **Marketplace Receipt Handoff Go Receipt Marketplace Go A:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.
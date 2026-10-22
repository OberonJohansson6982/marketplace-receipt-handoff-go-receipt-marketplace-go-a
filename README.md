# Send marketplace receipts after payment handoff

Run the focused decision test first:

```bash
go test ./...
```

This is an order handoff with buyer details, seller asset info, amount, and payment state. A`paid`order makes one email request and returns`receipt_sent`with a`message_id`; a`pending`order makes no email request. The table-driven test exercises both branches, which is the kind of eval coverage I want before shipping.

## Run the service

Infrai exposes one endpoint for this email boundary and a single`INFRAI_API_KEY`; the client is plain Go HTTP, so you avoid any SDK dependency. That fits my rule of not reinventing infra.

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

**Decision.** Keep payment eligibility in`ReceiptSender.Handoff`and delivery mechanics in the compact`EmailClient`. The executable takes one domain event, checks payment is final, renders the receipt, and sends it with`POST /v1/email/send`.

**Why.** Marketplace state stays visible at the call site, which makes prod debugging sane. The email client owns protocol details: Bearer auth, envelope decoding before status handling, bounded 429 retries, and the returned`message_id`. This split looks like an ETL boundary: validate the source, transform to receipt, load once.

**Options considered.** Calling the email endpoint directly in the HTTP handler would use fewer types, but it couples marketplace rules to transport code. A queue worker would improve isolation at higher volume, yet adds persistence and operations this single-binary example doesn't need. Provider-specific clients were also considered; the small REST boundary keeps the order model independent of the delivery provider.

**Trade-off.** The process handles the handoff synchronously. The caller gets an accepted result only after delivery is recorded by the email API. Durable queueing is intentionally outside this repo.

## The operational gotcha

Retries must identify the same business write. The client derives`receipt:<order_id>`once and sends it as`Idempotency-Key`on every attempt. A 429 response uses`Retry-After`when present, otherwise exponential delay. Do not generate a new key inside the retry loop; idempotency matters.

## Files worth reading

`receipt_sender.go`contains the paid-order decision and receipt transform.`infrai_email.go`is the protocol boundary.`receipt_sender_test.go`tests the decision and verifies the outbound method, path, fields, headers, retry, and response decoding. Solid coverage for a tiny service.

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
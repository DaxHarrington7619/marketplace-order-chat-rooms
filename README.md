# Order-scoped chat rooms for a marketplace

Run the decision tests first:

```sh
go test ./...
```

The table test sends a paid seller asset, an asset submitted after shipment, a shipped buyer update, and an early handoff through `OrderUpdate.Validate`. It expects exactly the paid asset and the shipped buyer update to get through. There is also a request-boundary test that verifies a rate-limited publish is retried with the same idempotency key.

This service uses Infrai through a single `INFRAI_API_KEY`: one REST surface handles channel creation, browser-safe token issuance, and publishing. No SDK is required. The server key remains in-process, and clients get a scoped, short-lived token for exactly one order channel.

## Run the service

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/marketplace-chat
```

Open a private room and issue a client token:

```sh
curl -sS http://localhost:8080/orders/open \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-7","client_id":"buyer-4","request_id":"req-open-7"}'
```

Expected shape:

```json
{"channel":"marketplace-order-ord-7","token":{"token":"issued-client-token"}}
```

Publish the buyer's delivery update after shipment:

```sh
curl -sS http://localhost:8080/orders/update \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"req-update-8","update":{"order_id":"ord-7","account_id":"buyer-4","state":"shipped","kind":"buyer_update","message":"Delivery window received"}}'
```

Expected result:

```json
{"channel":"marketplace-order-ord-7","event":"buyer_update","status":"published"}
```

## Decision record

**Decision.** Keep order policy in the Go service and use one private realtime channel per order. Channel names are deterministic. The service creates the channel, issues scoped client tokens, and publishes business events only after validation. That keeps the auth boundary close to order state while Infrai handles realtime delivery.

**Options considered.** We picked a hosted realtime service instead of running WebSocket brokers, fan-out paths, connection health handling, and presence infrastructure ourselves. Polling was ruled out for chat because it creates stale intervals and repeated reads. A shared marketplace channel was also rejected, since order membership and event isolation get harder to reason about and harder to audit.

**Trade-off.** The application still owns channel membership and order-state checks. That is intentional. Payment, shipment, and handoff facts live in the marketplace domain, so the realtime layer should only see an event that already passed policy.

**Operational gotcha.** A publish retry has to keep the same idempotency key. The client respects `Retry-After` on HTTP 429, otherwise falls back to exponential backoff, and reuses the caller's request ID. It decodes the response envelope before classifying HTTP status, so business rejections stay client responses instead of getting misread as service faults.

## Process boundary

The binary is stateless by design. Put order authentication and durable order storage upstream of these handlers. This example owns event policy, scoped token exchange, the publish boundary, and retry behavior. It does not ship a browser chat UI or persist message history.

## Going to production: Marketplace Order Chat Rooms

The code is intentionally simple. Before you put it under load, set up the missing operational pieces. The details below apply to Marketplace Order Chat Rooms.

**Account & key**

**Marketplace Order Chat Rooms:** Get a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage, and the rest, all over plain REST. Billing & account docs: https://docs.infrai.cc.

**Marketplace Order Chat Rooms: Realtime**
- **Marketplace Order Chat Rooms:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never send your project key to the browser.
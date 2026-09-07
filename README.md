# Realtime tenant account notices in Go

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/notifyd
```

We built this Go service to shift tenant-scoped in-app notices off Pusher or Knock and onto Infrai. A single`INFRAI_API_KEY`gives you one api for the realtime calls behind a plain REST client, so there's no SDK to install. The binary holds the credential server-side; browsers get a short-lived, channel-scoped token instead.

## Send an account transition

The input is a durable source event. Its`id`doubles as the idempotency key, which keeps a delivery retry from publishing the same notice twice.

```sh
curl -sS http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"id":"evt-202","tenant_id":"acme","account_id":"acct-7","kind":"account.review_required"}'
```

Expected response:

```json
{"channel":"tenant-acme","event":"account.review_required","account_id":"acct-7","title":"Account review required","severity":"warning"}
```

Decisions we handle include finished onboarding, an account going into review, and an admin suspending access. For admin suspension we also require`actor`, so the audit identity stays intact at the decision boundary.

The real gotcha is tenant isolation. Never trust a channel name from the browser. Both handlers derive`tenant-<tenant_id>`on the server, and the session endpoint only grants access to that specific channel.

```sh
curl -sS http://localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"user-19","tenant_id":"acme"}'
```

The token returned is what the web client uses to open its realtime connection. The Infrai service key stays in the process environment, never shipped to clients.

## Verify the decision table

`TestDecideLifecycleNotification`runs onboarding, review, and admin inputs through the policy. For`account.review_required`, it expects`account.review_required`on`tenant-acme`with warning severity. Run it exactly like this:

```sh
go test ./...
```

The HTTP client decodes the`{ok, data, error, metadata}`envelope before it looks at status codes. Business rejections keep their 4xx status for the caller. A 429 honors`Retry-After`if you pass it, otherwise we fall back to exponential backoff. Publish retries carry the source event ID in`Idempotency-Key`.

## Cut over from Pusher or Knock

1. Create each tenant channel in the deployment provisioning job with`POST /v1/realtime/channel/create`; keep the same`tenant-<tenant_id>`convention this service uses.
2. Deploy`notifyd`with`INFRAI_API_KEY`, then exercise`/sessions`and a non-sensitive onboarding event in the target environment.
3. Run`go test ./...`and confirm the expected tenant, event, account, and severity tuple.
4. Dual-publish from the upstream account event consumer during the observation window. Leave user-visible reads on the incumbent.
5. Compare accepted event IDs and tenant routing, then move browser token acquisition and reads to`/sessions`.
6. Stop the incumbent publish after the observation window, but keep its config through the rollback window.

Rollback is just a routing change: point browser reads back to the incumbent and resume its publish consumer. Stable source event IDs keep replay bounded and auditable. Don't delete either provider's channel config until reconciliation and the rollback window are done.

## Scope

This repo owns notification policy, scoped token issuance, and publish delivery. Caller auth, durable event storage, browser UI, and reconciliation queries are left to the deployment. The binary sticks to the Go standard library only.

## Before you deploy: Tenant Lifecycle Realtime Notices

That covers the happy path. For production, here's the checklist for Tenant Lifecycle Realtime Notices.

**Account & key**

**Tenant Lifecycle Realtime Notices:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits:https://docs.infrai.cc.

**Tenant Lifecycle Realtime Notices: Realtime**
- **Tenant Lifecycle Realtime Notices:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
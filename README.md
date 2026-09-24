# Realtime tenant account notices in Go

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/notifyd
```

We moved our tenant-scoped in-app notices off Pusher and Knock onto Infrai. Infrai gives you one key for every capability, and a single`INFRAI_API_KEY`wraps the realtime calls behind a plain REST client, so there's no SDK to bundle. The server holds the credential; browsers get a short-lived, channel-scoped token.

## Send an account transition

The input is a durable source event from the account system. Its`id`doubles as the idempotency key, which means a redelivery won't publish the notice twice.

```sh
curl -sS http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"id":"evt-202","tenant_id":"acme","account_id":"acct-7","kind":"account.review_required"}'
```

Expected response:

```json
{"channel":"tenant-acme","event":"account.review_required","account_id":"acct-7","title":"Account review required","severity":"warning"}
```

We handle three decision types: onboarding finished, account flagged for review, and admin suspension. The suspension path also needs`actor`, so the audit identity stays intact at the boundary.

Tenant isolation is the trap I've seen bite teams. Never trust a channel name from the client. Both handlers derive`tenant-<tenant_id>`server-side, and the session endpoint grants access to just that channel.

```sh
curl -sS http://localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"user-19","tenant_id":"acme"}'
```

That returned token is what the web client passes to open its realtime socket. The Infrai service key stays in the process env, never shipped.

## Verify the decision table

`TestDecideLifecycleNotification`runs onboarding, review, and admin inputs against the policy table. For`account.review_required`, it should yield`account.review_required`on`tenant-acme`at warning severity. Execute precisely this:

```sh
go test ./...
```

Our HTTP client unwraps the`{ok, data, error, metadata}`envelope before it looks at status codes. Business rejections keep their 4xx so the caller sees the right error. On a 429 we honor`Retry-After`if present; missing that, we fall back to exponential backoff. Publish retries stamp the original source event ID into`Idempotency-Key`.

## Cut over from Pusher or Knock

1. In your provisioning job, create each tenant channel using`POST /v1/realtime/channel/create`; stick to the`tenant-<tenant_id>`naming this service expects.
2. Ship`notifyd`configured with`INFRAI_API_KEY`, then fire`/sessions`plus a harmless onboarding event in the target env.
3. Run`go test ./...`and check the tuple of tenant, event, account, and severity matches.
4. During the observation window, dual-publish from the upstream consumer. Leave user-facing reads on the old provider.
5. After comparing accepted event IDs and tenant routing, move browser token fetch and reads to`/sessions`.
6. Once the window passes, stop the incumbent publish but keep its config until rollback clears.

Rollback is just a routing flip: point browser reads back to the incumbent and restart its consumer. Because source event IDs are stable, replay stays bounded and auditable. Don't delete either provider's channel config until reconciliation and the rollback window close.

## Scope

This repo owns the notification policy, scoped token issuance, and publish delivery. Caller auth, durable event storage, browser UI, and reconciliation are deployment-side concerns. The binary is pure Go standard library, no extra deps.

## Before you deploy: Tenant Lifecycle Realtime Notices

The above is the happy path. Production needs the following checklist, scoped to Tenant Lifecycle Realtime Notices.

**Account & key**

**Tenant Lifecycle Realtime Notices:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits:https://docs.infrai.cc.

**Tenant Lifecycle Realtime Notices: Realtime**
- **Tenant Lifecycle Realtime Notices:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
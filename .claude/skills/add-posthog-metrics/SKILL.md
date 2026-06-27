---
name: add-posthog-metrics
description: Use when adding PostHog product-analytics events or metrics to a feature in apps/platform — instrumenting clicks, form submits, or other interactions — or when gating an in-development feature behind a PostHog feature flag and measuring its usage. Covers event naming, consent-safe capture, privacy, and the client-vs-server split.
---

# Add PostHog metrics

PostHog is already wired in `apps/platform` (infra in `~/analytics`, consent in
`~/features/consent`). You add metrics by **capturing events from the client** — you do NOT
re-initialize PostHog and you do NOT add consent checks.

## Access PostHog

- **In a React component:** `usePostHog()` from `posthog-js/react` — the provider is mounted
  in `main.tsx`. It returns `null` when analytics is disabled (no `VITE_POSTHOG_KEY`, or in
  tests), so call through optional chaining.
- **Outside components** (class components, plain modules): `getAnalytics()` from `~/analytics`
  (also nullable). Precedent: `analytics/error-boundary.tsx`.

```tsx
import { usePostHog } from 'posthog-js/react';

function AgentCard() {
  const posthog = usePostHog();
  return (
    <Button onPress={() => posthog?.capture('agent_card_opened', { agent: 'scout', location: 'agents_section' })}>
      …
    </Button>
  );
}
```

## Capture — conventions

- `posthog?.capture('<object>_<verb>', { …props })`. Event names: **snake_case, `object_verb`,
  past tense** — `cta_clicked`, `search_submitted`, `tender_saved`.
- Always include a `location` prop naming the surface (`'hero'`, `'cta_band'`, `'search_dock'`).
- `locale` is already attached to every event as a super-property (registered by
  `useLocaleProperty`) — **do not duplicate it** in props.
- Property keys snake_case; values primitive.

## Consent is automatic — never gate capture yourself

PostHog is initialized `opt_out_capturing_by_default: true`; the cookie banner
(`~/features/consent`) calls `opt_in_capturing()` / `opt_out_capturing()`. Events captured
before consent are dropped by posthog-js. So **call `capture()` unconditionally** — wrapping it
in `if (hasConsent)` is wrong and redundant.

## Privacy — this is an EU product

No PII or business-sensitive free text in event properties. For a search query or tender
title, send `query_length` / a category / a hash, **not** the raw string. (Session replay
already masks inputs.)

## Gating an in-development feature (feature flags)

For a feature still in development, gate it behind a PostHog **feature flag** and capture its
usage so you can measure adoption before rolling it out to everyone:

```tsx
import { useFeatureFlagEnabled, usePostHog } from 'posthog-js/react';

function ExperimentalSearch() {
  const enabled = useFeatureFlagEnabled('experimental-search'); // create this flag in PostHog
  const posthog = usePostHog();
  if (!enabled) {
    return null; // hidden until the flag is turned on for this user
  }
  posthog?.capture('experimental_search_viewed', { location: 'search_dock' });
  return /* …the in-development UI… */;
}
```

Create the flag in the PostHog dashboard and roll it out to internal/beta users first. Delete
the flag **and** the `useFeatureFlagEnabled` guard once the feature ships to everyone.

## Server side = logs, not product events

The Go server's PostHog integration (`internal/telemetry`) ships **`slog` logs** via OTLP, not
product events. For a server-side metric, add a `slog.InfoContext(ctx, "…", slog.Int(...))`
call — it reaches PostHog logs when `POSTHOG_API_KEY` is set. Do **not** add a product-analytics
SDK to Go for these.

## Common mistakes

| Mistake | Do instead |
| --- | --- |
| `import posthog from 'posthog-js'` / `posthog.init(...)` in a component | `usePostHog()` — the provider is already mounted |
| `if (hasConsent) posthog.capture(...)` | Call `capture()` unconditionally — opt-out gating is automatic |
| Capturing a raw query / tender title / email | Send length / category / hash; no PII |
| Adding a Go product-analytics client for a metric | `slog` → OTLP logs is the server path |
| Inventing an event-name style per feature | snake_case `object_verb`, always with a `location` prop |
| Duplicating `locale` in event props | It's already a super-property |

---
name: deadline-reminder-pipeline
description: "core/alerting deadline reminders — 14/7/3/1-day buckets plus a per-bid watermark ARE the idempotency mechanism (no lock), bucketFor must return the narrowest crossed threshold, daysUntil truncates on purpose, and which orchestration outcomes advance the watermark"
metadata:
  type: project
  updated: 2026-08-22
  sources: [docs/superpowers/prd/2026-08-17-competitive-program.md]
---

`services/backend/internal/core/alerting` is the first domain in this codebase whose job is to
**reach out** — everything the mail adapter shipped before was authentication or invitation.
Deliberately a *deadline reminder on a gara the user themselves tracked*, not a "new matches"
digest: a digest competes with a free one from TenderWolf and is judged on match precision
from the first send, while a reminder competes with nothing, carries no precision risk, and
points at something real.

## Bucketing is the whole idempotency mechanism

`var buckets = []int{14, 7, 3, 1}` (days) plus a per-bid watermark
(`bids.last_reminded_bucket`). An hourly job reminding on "deadline is near" sends 24 mails a
day; bucketing turns the countdown into **four discrete events**, so a bid gets at most four
mails over its whole life however often the job runs or retries — **with no lock anywhere**.

- **`bucketFor` returns the NARROWEST threshold already crossed, not the widest**, and this is
  the difference between working and silently not working. At seven days a bid has entered
  both the 14 and the 7 bucket; returning 14 compares equal to a watermark already at 14, so
  the seven-day reminder **would never fire at all**. It scans from the tightest threshold
  upward. It also means a job that slept through thresholds fires the bucket the bid is
  *actually* in — at two days with a watermark of 14 it returns 3: one mail, not two.
- **`MarkReminded` uses `LEAST`** so the watermark only ever moves *toward* the deadline; an
  out-of-order pass cannot widen it back and re-arm a bucket already sent. Tested in all three
  directions.
- **`daysUntil` truncates on purpose.** A deadline 30 minutes out truncates to 0 — "oggi", the
  last call — where rounding up would say "domani" and grant a day the reader doesn't have.
  The cost, found by running it against a real database rather than by reading it: 47 hours
  also truncates to 1, so the mail says "domani" when it is nearly two days away. The number
  is **understated, never overstated** — the reader is told they have *less* time and acts
  earlier, which for a legal submission deadline is the only safe direction of error.

## Two orchestration calls that are easy to get backwards

- **Nobody left to mail → ADVANCE the watermark.** Everyone opted out; leaving it would make
  the job reconsider that bid on every pass until its deadline, for no one.
- **Every send failed → do NOT advance**, so the next pass retries. The one case where a
  duplicate is the lesser risk, against a bucket silently lost to an outage.

Suppressions (closed, submitted, decided `no_go`, no published deadline, deadline passed,
already reminded) are **silent by design** — none is an error, they are the normal state of
most bids most of the time, and logging them would drown the one signal worth reading. Only an
*explicit* `no_go` suppresses: an undecided bid is exactly the one a deadline should chase.

## Storage and the query

- The watermark lives **on the bid**, not in its own table: one small integer with the same
  lifetime as the row, where a join table would be a second place for the same fact to be
  wrong.
- `unsubscribe_token` is stored **in plain**, unlike auth's hashed tokens — a threat model, not
  an inconsistency. See [[marketing-email-deliverability]].
- The candidate SQL is a **coarse filter on purpose**: it excludes only the two states that can
  never become due again plus a 30-day horizon deliberately wider than the widest bucket.
  `alerting.Due` owns the rule; encoding bucket arithmetic in SQL would put it in two places,
  and SQL is the copy nobody thinks to check when thresholds change.
- **A member with no preferences row is a SUBSCRIBER** — that's what the `LEFT JOIN` buys. They
  have never been asked, and the first reminder is where they get the chance to say no. An
  `INNER JOIN` would exclude everyone until a row existed, which reads as "nobody wants
  reminders" and is indistinguishable from a broken query.
- Tokens are minted **one at a time** with their own `crypto/rand` value, never in one bulk
  INSERT: a single token shared across a workbench would let any member unsubscribe every
  other. Minting is idempotent and a test pins that a second pass does **not** rotate an
  existing token — rotation would silently break the link in every mail already delivered.

## The digest binary and its CronJob

`services/backend/cmd/digest` — one pass, then exit, in the **backend image** with the CronJob
overriding the entrypoint (the shape `services/ingestion` already uses for its binaries). Not
a new service (the microservices threshold in [[system-design-principles]] is unmet), not in
ingestion (it needs this module's `internal/` packages), and **not an in-process ticker** (N
replicas would send N copies, and cadence would couple to pod lifecycle).

- `infrastructure/kubernetes/tendersbay-xyz/backend/main/digest-cronjob.yaml`, registered in
  `kustomization.yaml`. Schedule `0 6 * * *` — **daily, not hourly, for product reasons not
  load**: buckets already cap a bid at four mails, so hourly would send the same four, just at
  whatever hour the countdown crossed a threshold. 06:00 UTC is breakfast in Rome either side
  of DST, and early enough that a 1-day last call still leaves a working day to act on.
- **`backoffLimit: 0`, deliberately.** A pass is not idempotent in the direction a retry needs
  — the watermark advances as mail goes out — so a killed job has already delivered part of
  its batch, and the next daily fire picks up the rest without a second process racing it.
- **Unconfigured reminders exit 0, not 1.** That is a choice someone made; a CronJob failing
  daily over it would page them about their own decision.
- The API process builds the alerting service with a **nil mailer** on purpose: it serves the
  unsubscribe endpoint and never sends, so someone who received a reminder last month can
  still escape it after sending is switched off.

## `DIGEST_DRY_RUN`, and the deploy burst it exists for

`last_reminded_bucket` defaults to **0**, so the first pass after deploy treats every existing
bid as never reminded and mails every one already inside 14 days of its deadline. That is
correct — those deadlines are real and nobody was told — but it is a **burst**, and the
difference between expecting it and discovering it is being able to count it beforehand.
`DIGEST_DRY_RUN` decides what a pass would send and sends nothing; it needed no mailer and no
dry-run mode in the domain, because `Due` is already a pure public function, so the dry run
reads candidates, applies the rule, and stops before the part that acts. It logs **counts per
bucket only** — no bid ids, no addresses — because it is something an operator runs against
real data to answer "how many", so it is safe to point at production.

Reminder-click attribution (`reminder_link_opened` + the bucket that sent it) is in
[[landing-analytics]]; recipient language in [[user-locale-capture]]; verifying all of this
against a real database in [[sandbox-verification-capabilities]].

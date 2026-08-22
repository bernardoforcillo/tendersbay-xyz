---
name: marketing-email-deliverability
description: "Transactional and marketing mail must not share a sending domain — enforce it in code, not docs; List-Unsubscribe + One-Click on every message; GET on /unsubscribe must change nothing because scanners prefetch links"
metadata:
  type: reference
  updated: 2026-08-22
  sources: [docs/superpowers/prd/2026-08-17-competitive-program.md]
---

Deliverability rules learned building the deadline reminders
([[deadline-reminder-pipeline]]). They generalise to any product that starts sending
non-authentication mail.

## Separate the sending domain, and enforce it in code

Sent from one domain, transactional and marketing mail **share a reputation**: a single spam
complaint about a deadline reminder degrades delivery for password resets — the mail a
locked-out user cannot do without. So reminders are a **separate sender type** with its own
`From`, not five more methods on the existing transactional sender.

**The separation is enforced, not documented.** `email.NewReminder(apiKey, from,
transactionalFrom, …)` **refuses to build** when `domainOf(from) == domainOf(transactionalFrom)`.
Reusing `MAIL_FROM` for both is the likeliest way this protection is lost, and it would be lost
**silently** — discovered later, when password-reset mail starts bouncing. A config invariant
whose violation is invisible until an unrelated system degrades belongs in a constructor, not
in a runbook.

Two details that keep the guard honest:

- The **test-only** constructor (`NewReminderWithURL`) applies the same check. A test helper
  that skipped it would let the invariant rot behind green tests.
- `domainOf` returns `""` for an address with no `@`, so an unparseable address fails the
  equality check **open** rather than blocking startup on a format the function doesn't
  understand. (It handles the `Name <box@domain>` display-name form too.)

## Every message carries an escape hatch

- `List-Unsubscribe: <url>` **and** `List-Unsubscribe-Post: List-Unsubscribe=One-Click`
  (RFC 8058), plus a visible link in the body. The `-Post` pair is what makes the mail
  client's *own* unsubscribe button work without the reader opening the message — which is
  what keeps a complaint from becoming a spam report.
- A **tokenless recipient is refused at the sender layer too**, not only in the domain. Defence
  in depth belongs specifically at the layer that would actually put an inescapable message on
  the wire. A message with no way out is both unlawful in the EU and the fastest route to
  password-reset mail being filtered as spam.

## GET must change nothing; POST performs

Mail scanners and link prefetchers follow **every** URL in a message. So:

- **`GET /unsubscribe` renders a confirmation form and changes nothing.** A GET that acted
  would opt out people who never clicked, and the symptom — reminders silently stopping for
  users who still wanted them — is close to undiagnosable.
- **`POST /unsubscribe` performs it**, and is also the RFC 8058 one-click target. That is why
  the token is read from the **query string**: the body belongs to the mail client, the URL is
  ours.
- **An unknown token renders the same page as a known one.** Distinguishing them would turn an
  unauthenticated endpoint into an oracle for which tokens are live; a miss is logged instead.
- **A failure is never reported as success.** Telling a reader they are unsubscribed while mail
  keeps coming is the one outcome this endpoint must not produce, so a store error and an
  unwired service both return 5xx and say so.
- **Opt-out is a timestamp, not a boolean.** `opted_out_at` records *when*, which is what a
  deliverability complaint is answered with.

## Token storage: hash auth tokens, store this one in plain

An auth token grants access to an account, so the server must never be able to reproduce one —
hash it. An **unsubscribe** token grants exactly one power ("stop sending me these"), has to
travel in every message, and must still work in a mail opened months later, which a hash
cannot do. A leak buys silencing a person's own reminders; regenerating the column revokes it.
Storing the two differently is a threat model, not an inconsistency — say so at the schema, or
someone will "fix" it.

## Copy: don't machine-translate a legal deadline

Reminder copy is **Italian and English only**, even though the frontend ships 24 locales. A
machine-translated reminder about a legal deadline is not honest, so everything else falls back
to English deliberately. (Italian names *domani*/*oggi* rather than counting them.) Which
language a given user gets is [[user-locale-capture]].

**Templating gotcha:** `html/template` entity-encodes `+` to `&#43;` inside an `href`, which is
correct — the parser decodes it before the URL is used. A test comparing the raw href would
fail on a link that works; decode before asserting.

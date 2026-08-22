---
name: user-locale-capture
description: "users.locale — an empty string must stay representable (stored locale = a choice, English = what a renderer does when nobody chose), signup already carried the locale and discarded it, EnsureLocale backfills on login but never overwrites"
metadata:
  type: project
  updated: 2026-08-22
  sources: [docs/superpowers/prd/2026-08-17-competitive-program.md]
---

The backend had no per-user language until 2026-08-22, so every reminder
([[deadline-reminder-pipeline]]) fell back to English. `users.locale` (declared in
`services/backend/internal/adapter/postgres/schema.go` as `pg.Text("locale").NotNull().Default("''")`)
closes it. Domain: `services/backend/internal/core/auth/locale.go`.

**The locale was already being collected and thrown away.** `SignUpRequest.locale` had existed
all along, and `auth.Service.SignUp` used it *only* to build the verification link before
dropping it — every user in the system had told us their language and nothing kept it. One
line stores it now. Worth generalising: before adding a capture path, check whether the value
already arrives and is discarded.

## `""` must stay representable — do not default it to English on storage

A stored locale means **somebody told us**. English is **what a renderer does when nobody
did**. Collapsing the two makes an explicit English choice indistinguishable from never having
been asked, which removes any way to find the users still worth asking. The verification
**link** does need a path segment, so it defaults *there and only there* — the two uses diverge
deliberately, and that divergence is the point rather than an inconsistency to tidy up.

## Two capture paths, neither needing a proto change

- **Signup** persists the form's locale, falling back to the browser's `Accept-Language` when
  the client sends none. The header is read in the **connectapi handler**, not the domain —
  HTTP is the adapter's business ([[code-organization-principles]]).
- **Login backfills** anyone created before this existed: the one moment with both a known user
  and a live browser header, costing a read that has just happened anyway. A failure is
  **logged, never returned** — nobody should fail to sign in because we could not record their
  language.

## `EnsureLocale` never overwrites, and that has its own test

An `Accept-Language` header describes the **device** someone happens to be holding; a stored
locale is a **choice**. Letting the header win would flip an Italian user's language because
they opened the app on a work laptop set to English, and they would have no idea why their
reminders changed. `EnsureLocale` returns early if `u.Locale != ""`.

`NormalizeLocale` maps bare and regional tags onto the 24 shipped builds (`it-CH` and `pt-BR`
are Italian and Portuguese to a product that ships one of each) and returns `""` for anything
else, so a garbage tag can never reach a renderer that has no copy for it. The 24-locale
frontend side of this is [[locale-namespace-insertion]].

**Still blocked:** changing the language from account settings needs
`UpdateProfileRequest.locale`, which needs protobuf codegen — impossible in this sandbox
([[sandbox-verification-capabilities]]). Storage, validation and both capture paths are done;
only the settings control is missing.

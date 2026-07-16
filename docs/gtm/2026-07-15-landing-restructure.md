# Landing restructure — competitor-informed, category-defining

Point-in-time strategy (July 2026). Competitive snapshot of the EU
procurement / bid-intelligence landscape, the structural patterns every player
shares, the gaps nobody fills, and the restructured landing architecture that
wins on the gaps. Positioning and terminology law stay in
`.claude/memory/landing-page-design.md` — this doc maps competition + behavior
onto that positioning; it does not fork it.

Working language English. Copy ships per-locale in `apps/platform`. No invented
numbers: every figure below is sourced.

**Conversion mechanism: DIRECT SIGNUP.** The product is real and shipped —
auth/signup/login/verify, account, workspace, workbench, explore. The landing's
job is to drive signup to the existing `/$locale/auth/signup` flow (the CTA band
already links there). This is **not** a waitlist page; no waitlist form, no
"claim your spot" copy. Pre-launch honesty applies to **proof only** — no fake
logos, counts, testimonials, or win rates.

---

## 1. Competitive landscape — how the category positions and structures

Two archetypes plus the institutions.

**A. Modern SaaS "bid intelligence" (the ones with real landing craft):**
Stotles (UK), Tendium (Nordics), Mercell (Nordics/EU), Tussell (UK).

**B. Legacy national alert services (functional, no swagger, single-country):**
Telemat, InfoAppalti, InfoBandiPA, Banchedati (IT); and the AI-first outlier
FareAppalti.AI (IT). Full IT treatment already in
[italian-keyword-map.md](italian-keyword-map.md).

**C. Institutions (unbeatable, not competitors — they are the source of truth):**
TED (Tenders Electronic Daily), national portals (TenderNed, eTenders, MEPA…).
Documented in [eu-head-terms.md](eu-head-terms.md).

### Structural read of the archetype-A landings (verbatim where quoted)

| Player | Hero headline | Proof strategy | Primary CTA | Coverage frame | Audience |
| --- | --- | --- | --- | --- | --- |
| **Stotles** | "One B2G platform built to find expiries · create target lists · qualify bids · win more contracts" | 15+ enterprise logos (SAP, Salesforce, Splunk); stat bar "8M+ notices, 15K+ buyers, 180K+ contacts"; SOC2/GDPR badges; named testimonial (Workday) | "Book a call" / "See it in action" (demo) | UK only | Public-sector **sales** teams, BD, RevOps |
| **Tendium** | "Intelligent B2G made for winners." | Stat bar "1,395+ customers · 1M tenders · €1.48B payment data"; 3 named testimonials + case studies | "Start free trial" / "Book demo" | Full Nordics; rest of EU only above-threshold | SMEs **and** enterprises (SME page refuses the SME-vs-giant frame) |
| **Mercell** | "Find, qualify, and win public tenders" | "Europe's most comprehensive database"; "5,000 public entities, 400,000 suppliers, €200bn" | "Start your free trial · 14 days, no credit card" | "Europe's most comprehensive" (no countries named) | Suppliers with a bid function |
| **Tussell** | "Trusted insights on government contracts and spend" | "80,000+ decision-makers"; analyst reports; buyer logos | "Book a demo" | UK only | BD & sales "trusted by leading suppliers to government" |

### The category conventions (the PATTERN — what everyone does)

1. **Hero = the same feature-verb list.** find · qualify · win / find & win. The
   whole category leads with an identical workflow noun-string.
2. **Proof = big proprietary numbers + enterprise logos + named testimonials +
   SOC2/GDPR badges.** This is the category's entire trust engine.
3. **CTA = "Book a demo" or "14-day free trial, no credit card."** Every player
   funnels to a live product / sales call.
4. **Audience = teams that already bid** — "public-sector sales", BD, RevOps,
   "grow pipeline", "win *more*". They sell *more* to the already-equipped.
5. **"B2G / public-sector sales" jargon.** Enterprise register.
6. **Coverage = scale/completeness flex**, single-country or Nordic-first in
   reality; EU as "above-threshold monitoring". No true pan-EU SME depth.
7. **Neutral-optimistic productivity tone** — "winners", "smarter", "grow". No
   villain, no loss frame.

### The gaps (what NOBODY does — tendersbay's wedge)

1. **Nobody speaks to the SME *without* a bid office.** The entire category
   sells to teams that already bid and want to bid more. Tendium's own SME page
   explicitly *refuses* the SME-vs-giant frame. The locked-out SME/entrepreneur
   is an empty position. **This is the wedge.**
2. **Nobody is agent-native.** They are all AI-*assisted tools* — smart filter,
   AI summary, copilots ("Louie"), AI matching. The human still drives. "A team
   of agents that does the work while you sleep" is a categorically different
   promise: done-for-you, not a better cockpit.
3. **Nobody uses loss aversion / the rigged-game frame.** All neutral-optimistic.
   The sharpest emotional lever in the category is unclaimed.
4. **Nobody is genuinely pan-EU for SMEs in 24 languages.** Confirmed by every
   source. "27 countries, one search, in your language" is unowned.
5. **Nobody makes the paperwork (ESPD/DGUE) the hero.** They stop at "bid
   assistance". The bureaucracy that actually breaks SMEs is under-served.
6. **Nobody pre-empts the AI-trust objection** (data-training, hallucination,
   per-client isolation) — timely given the ANAC / EU AI-Act "declare AI use"
   wave (see [italian-keyword-map.md](italian-keyword-map.md) §1.3).

### The strategic thesis — "spaccare" = win on the gaps

> **tendersbay is not a better tender tool. It is the anti-bid-office.**
> Where the category sells more pipeline to teams that already win, tendersbay
> hands a bid office to the SMEs who were locked out. Every competitor is a
> cockpit you fly; tendersbay is autopilot that flies while you sleep.

And the honesty judo: the category proves itself with numbers tendersbay
**cannot and must not fake** (customer counts, logos, win rates). So tendersbay's
proof comes from a source competitors can't copy and can't inflate — **the
verifiable public reality of the prize** (real EU procurement scale, real
directives, real portals, real deadlines). Proof of the *prize*, not proof of
*us* — and that proof also justifies signing up *now*: this much is on the table
and you see almost none of it.

---

## 2. The restructured landing architecture

Design language is unchanged (warm cream/green palette, the kit, the coverage
marquee, the deliberate light/dark value ladder — see
`.claude/memory/landing-page-design.md`). This is a **structure + positioning +
proof-strategy** restructure, not a visual redesign. Where a change touches the
deliberate section rhythm, it is flagged for the designer, not forced here.

### New section order (purpose · neuro rationale · how it beats the category)

1. **Hero** *(keep copy; re-point the primary CTA to signup)*
   Loss-framed headline ("The tender they already counted as theirs? Awarded.").
   The primary CTA ("Put your agents to work") should link to
   `/$locale/auth/signup` — the money action from the most valuable real estate.
   *Neuro:* loss aversion + isolation effect on one CTA. *Beats:* the whole
   category's neutral "find/win" hero — tendersbay opens on the wound.

2. **Proof-of-the-prize strip** *(NEW — the signature move)*
   A sourced stat band occupying the slot where competitors put logos + "8M
   notices in *our* DB". tendersbay has zero public customers to flex, so it
   flexes the *prize* instead: **€2 trillion+/year, 250,000+ public buyers,
   ~800,000 tenders/year** — all EU/TED-sourced, with a visible citation
   footnote.
   *Neuro:* loss aversion (money on the table you never see) + authority (real EU
   institutions) + processing fluency (three round numbers). The visible source
   line is itself the trust signal — "we cite, we don't invent" — the exact
   opposite of fake logos, and it earns the signup ask. *Beats:*
   Stotles/Tendium's proprietary-scale flex by making the scale about the
   **buyer's missed money**, not the vendor's database — un-fakeable because it
   is public record.

3. **Problem — the rigged game** *(keep)*
   Three cards: buried across 27 countries · drowning in paperwork · no bid
   office, no shot. *Neuro:* loss aversion + rule of three. *Beats:* nobody else
   names a villain; the "no bid office" card lands the wedge (gap 1) hardest.

4. **Agents — reframed as the tools-vs-agents wedge** *(RESTRUCTURE)*
   Same three agents (find · prepare · win), but a new **lead line** turns a
   feature list into a category redefinition: *"Everyone else sells you a faster
   search box. You still do the work. We send a team that does the work — find,
   prepare, win — while you run your business."*
   *Neuro:* categorization by contrast (buyers grasp a new thing against a known
   category) + done-for-you framing. *Beats:* Stotles/Tendium/Mercell, all of
   whom are AI-*assisted tools* where the human still drives (gap 2).

5. **Audience — three persona cards** *(keep)*
   run the bids · own the number · multiply across clients. *Neuro:*
   self-selection / identity. *Beats:* the category's generic "sales teams".

6. **Assurance — the AI-trust Q&A** *(keep)*
   data-training, hallucination, per-client isolation, integration. *Neuro:*
   objection pre-emption (peak-end: remove the last doubt before the ask).
   *Beats:* nobody else answers these, and the AI-Act/ANAC wave makes them urgent
   (gap 6).

7. **Coverage — the honest rolling map** *(keep)*
   27 flags, country-by-country rollout. *Neuro:* Von Restorff (only mid-page
   dark beat) + honest scarcity (your country isn't lit *yet*). *Beats:*
   Mercell's vague "Europe's most comprehensive" with a concrete, honest map
   (gap 4).

8. **Vision** *(keep)* — "public money behind a velvet rope; we're cutting it".
   *Neuro:* mission / meaning; airy light beat before the close.

9. **CTA — direct signup** *(RESTRUCTURE: waitlist → signup)*
   Drop the waitlist copy ("Join the waitlist", "Claim your spot"). The band
   already routes to `/$locale/auth/signup`; the copy now matches: *create your
   account and put the agents on your market.* *Neuro:* single clear action
   (isolation effect) + peak-end close. *Beats:* the category's "book a demo /
   14-day trial" gate — tendersbay's signup is a lower-friction, real front door;
   no sales call, no credit card wall.

### What to keep / restructure / add / cut

| Section | Verdict | Action |
| --- | --- | --- |
| Hero | **Keep copy; re-point CTA** | Primary CTA → `/$locale/auth/signup` (signup), matching the cta-band RouterLink pattern; keep secondary as in-page scroll. |
| Proof-of-the-prize | **ADD** | New `landing.proof` block + `proof-strip` organism after Hero. |
| Problem | **Keep** | No change (already lands the wedge). |
| Agents | **Restructure** | Add `landing.agents.lead`; render it as the section's lead. |
| Audience | **Keep** | No change. |
| Assurance | **Keep** | No change. |
| Coverage | **Keep** | No change. |
| Vision | **Keep** | No change. |
| CTA | **Restructure** | Revise `landing.cta.body` + `landing.cta.button` from waitlist → signup. |
| Cut | **Nothing** | The page is already tight; "spaccare" = sharper + a signature proof beat + a real signup close, not longer. |

---

## 3. Source copy (en-ie) for the build

Authored in `en-ie` (source locale); propagate to the other 23 per
[eu-head-terms.md](eu-head-terms.md). Stat **values** are universal (do not
translate the numbers); only **labels** and prose translate.

### New block — `landing.proof`

```json
"proof": {
  "lead": "Over two trillion euro of public money is spent across Europe every year. Most of it moves through tenders you never see.",
  "items": [
    { "value": "€2 trillion+", "label": "public spend a year across the EU" },
    { "value": "250,000+", "label": "public buyers putting work out to tender" },
    { "value": "~800,000", "label": "tenders a year — no team reads them all" }
  ],
  "source": "European Commission · TED (Tenders Electronic Daily)"
}
```

### New key — `landing.agents.lead` (insert alongside existing `title`/`items`)

```json
"lead": "Everyone else sells you a faster search box. You still do the work. We send a team that does the work for you — find, prepare, win — while you run your business."
```

### Revised — `landing.cta` (waitlist → signup)

```json
"cta": {
  "title": "Your agents are ready. The only thing missing is you.",
  "body": "Create your account and put the agents on your market — find, prepare, win, without a bid office of your own.",
  "button": "Create your account"
}
```

Sourcing for the proof numbers (put nothing on the page that isn't here):

- **€2 trillion+/year, ~14% of EU GDP, 250,000+ public buyers** — European
  Commission, Public procurement (single-market-economy.ec.europa.eu). The
  Commission cites 14–16% of GDP / €2–2.5tn; "€2 trillion+" is the conservative
  floor. Use "250,000+" for contracting authorities.
- **~800,000 notices/year, worth €815bn+** — TED / OJ S official statistics
  (ted.europa.eu). Use "~800,000".

Numbers are stated as **round, conservative, sourced** figures with a visible
attribution line. Never sharpen them past the source.

---

## 4. Scope & honesty notes (read before building)

- **CTA drives signup, not a waitlist.** `cta-band/index.tsx` already routes to
  `/$locale/auth/signup`. Do **not** add a form or invent "N people ahead of
  you". Only the copy changes (waitlist → signup) per §3.
- **Coverage stays honest.** The app is live and signup works, but tender
  *coverage* still rolls out country-by-country (the marquee's `AVAILABLE` set is
  empty). The CTA promises an *account*, not that your country is lit tonight —
  no contradiction, and no country-timing claim in the CTA copy.
- **Section rhythm (reconciled against the code).** In the current
  `redesign-explore` code the ladder is Hero (cream-100) → **ProofStrip
  (cream-100, seamless)** → Problem (**dark**, `bg-ink-900`) — so the proof strip
  continues the hero's cream field and the dark Problem band provides the value
  drop, no seam break. NB: the `.claude/memory/landing-page-design.md` note that
  reads "Problem L · Agents D" is now **stale** relative to this code (Problem is
  dark); worth a librarian/designer reconciliation, not a "fix" here.
- **No visual redesign.** If a deeper treatment of the proof strip is wanted
  (e.g. animated counters), that is a designer call — flag it, don't build blind.

---

## 5. Implementation handoff — for gtm-engineer

Execute in the worktree at
`c:\Users\berna\Desktop\Github\tendersbay-xyz\.claude\worktrees\agent-acf7c2961cd4f2ab7`.
Do **not** commit, tag, or publish. Copy-and-structure only.

1. **Add the `landing.proof` block** (verbatim en-ie copy from §3) to
   `apps/platform/src/assets/locales/en-ie/common.json` under `landing`, placed
   after `hero`.
2. **Add `landing.agents.lead`** (verbatim from §3) to the `en-ie` `agents`
   block alongside `title`/`items`.
3. **Revise `landing.cta`** in `en-ie` per §3: keep `title`; replace `body`
   (drop "switching on the EU market / join the waitlist / country lights up")
   with the signup body; change `button` from "Claim your spot" to "Create your
   account".
4. **Propagate steps 1–3** to all 23 other locales'
   `src/assets/locales/<locale>/common.json`. Translate `lead`, `label`,
   `source`, and the CTA `body`/`button` prose; **keep the stat `value` figures
   identical** across locales (numbers are universal — leave "€2 trillion+",
   "250,000+", "~800,000" as-is). Use each market's native procurement
   vocabulary from [eu-head-terms.md](eu-head-terms.md) where prose names
   "tenders". Any locale whose existing CTA `button` was the localized "Claim
   your spot" must move to that locale's "Create your account" equivalent.
5. **Build a `proof-strip` organism** at
   `src/features/landing/components/organisms/proof-strip/` (reuse the mono-label
   + value pattern; JetBrains Mono for the numbers per the type system). Render
   `landing.proof.lead`, the three `items` (value + label), and a small `source`
   footnote. Read the array with
   `t('landing.proof.items', { returnObjects: true })`. Light band (cream) that
   continues the hero field — see §4 rhythm note. Export it from the organisms
   barrel.
6. **Insert `<ProofStrip />` in the landing template**
   (`components/templates/landing-template/index.tsx`) **between `<Hero />` and
   `<ProblemSection />`**.
7. **Render `landing.agents.lead`** as a lead paragraph in
   `components/organisms/agents-section/index.tsx` (above the three agent cards,
   below the section title; brand-50 text on the brand-700 background).
8. **Re-point the hero primary CTA to signup** in
   `components/organisms/hero/index.tsx`: the primary `Button`
   (`landing.hero.ctaPrimary`, "Put your agents to work") should navigate to
   `/$locale/auth/signup` (use the TanStack `RouterLink` + `params={{ locale:
   i18n.language }}` pattern from `cta-band/index.tsx`; extend the landing
   `Button` atom to support a router link if needed). Keep the secondary CTA as
   the in-page scroll it is today. If extending the `Button` atom is more than a
   trivial change, leave the hero CTA as a scroll and note it — do not block the
   rest of the build on it.
9. **Extend the completeness test**
   `src/assets/locales/landing-content-keys.test.ts` to assert, across all 24
   locales: `landing.proof.lead` (string), `landing.proof.items` (array of 3,
   each with non-empty `value` + `label`), `landing.proof.source` (string), and
   `landing.agents.lead` (string). Optionally assert `landing.cta.button` is
   present/non-empty (it already is) to lock the signup label in.
10. **Run focused checks** (env gotcha — call binaries directly):
    `apps/platform/node_modules/.bin/vitest run --root apps/platform` scoped to
    `landing-content-keys.test.ts` (plus any proof/agents render test), and
    `node_modules/.bin/biome check --write` on the touched files only. Report
    pass/fail. **No commit.**

Acceptance: all 24 locales carry `landing.proof.*` (3 items) + `landing.agents.lead`
and a signup-oriented `landing.cta` (no waitlist wording anywhere); completeness
test green; Biome clean on touched files; `ProofStrip` renders after the hero;
the CTA (and, ideally, the hero primary) drive `/auth/signup`; no fabricated
numbers (only the three sourced figures in §3).

---

## 6. Agents section — hook rethink (follow-up, July 2026)

The agents section shipped as a solid but conventional three-step feature
triptych (Find · Prepare · Win) under a descriptive headline. The ask: rework it
through a **hook lens** — grab attention and pull the reader in, *show* the agents
in action rather than describe them, and keep the tools-vs-agents wedge but lead
with the hook, not the category contrast.

### Candidate hook mechanisms

**A. The overnight shift — a time-stamped micro-scenario (open loop).** *(chosen)*
Headline is an open loop / curiosity gap — "Here's what your agents did while you
slept." — and the three cards become three timestamped *moments* of one tender's
journey through a single night (02:14 found · 05:30 prepared · 07:00 you wake up
in the running). *Neuro:* Zeigarnik / open loop (an unresolved "what happened
overnight?" pulls the reader to the 07:00 payoff) + concreteness & narrative
transportation (a specific 2am moment is processed and remembered far better than
abstract verbs — "show, don't tell") + loss aversion / competitive tension (the
giants pay a whole office for this; you don't) + peak-end (the "you woke up in the
running" resolution). Keeps find/prepare/win as the spine (same 3 cards, same
icons) so it is a low-risk build, while transforming the *feel* from feature-list
to story. Translates cleanly — the 24h timestamps are universal.

**B. Pattern-interrupt question.** Open on a confrontational question — "It's
2am. A tender that fits you closes in nine hours. Who's working it?" — then answer
with the agents. *Neuro:* pattern interrupt + curiosity. *Rejected:* the
question→answer trope is still *telling*, not showing the agents at work, and it
risks reading as a gimmick; weaker on the "watch them work" ask.

**C. Before/after split (old way vs the agents).** "The old way: you, three Excel
tabs, midnight. tendersbay: the agents, already three tenders deep." *Neuro:*
contrast + anchoring against the SME's real current cost. *Rejected:* it *leads*
with the category contrast, which the brief explicitly says to avoid — the wedge
should stay, but the hook should lead. Also less "show the agents in action."

### Why A beats the current triptych

The current headline ("Three agents. They don't miss, don't sleep, don't quit.")
*asserts* a trait; A *demonstrates* it by making you watch a night unfold. An open
loop out-pulls a declarative headline (the reader keeps going to close the loop),
and a concrete 2am scene out-remembers three abstract labels. The tools-vs-agents
wedge survives — it moves into the lead line ("not a faster search box you still
have to drive") — but it no longer opens the section; the hook does.

### Source copy (en-ie) — revised `landing.agents`

Each `items` entry gains a `time` field (a mono eyebrow). Keep the values
identical across all 24 locales (24h clock is universal, like the proof stats);
translate only `title`/`body`/`lead`.

```json
"agents": {
  "title": "Here's what your agents did while you slept.",
  "lead": "Not a faster search box you still have to drive. A team of agents that works the whole tender — find, prepare, win — overnight, so you wake up already in the running.",
  "items": [
    {
      "time": "02:14",
      "title": "It found the one that fits",
      "body": "While the market slept, the agents swept all 27 countries and pulled the single tender worth your time — matched to what you actually do, not a keyword dump."
    },
    {
      "time": "05:30",
      "title": "It built the paperwork",
      "body": "Requirements read, certificates matched, the ESPD assembled, every deadline logged. The bureaucracy that used to burn your weekend — done before the coffee."
    },
    {
      "time": "07:00",
      "title": "You woke up in the running",
      "body": "No blank page. The spec turned into a bid strategy, ready for your call — the same firepower the giants pay a whole office for, waiting on your desk."
    }
  ]
}
```

Tone check: cutting toward the rigged status quo and the giants' bid offices, never
the reader; tender/procurement-specific (tender, ESPD, 27 countries, spec, bid
strategy, deadlines); no invented numbers.

### Implementation handoff (agents hook)

1. Replace `landing.agents.title`, `landing.agents.lead`, and rewrite the three
   `landing.agents.items` in `en-ie` per the copy above; **add a `time` field** to
   each item.
2. Propagate to all 23 other locales — translate `title`/`body`/`lead`, **keep the
   `time` values identical** (`02:14` / `05:30` / `07:00`). Use each market's native
   procurement vocabulary from [eu-head-terms.md](eu-head-terms.md) and its native
   ESPD acronym (it DGUE, fr DUME, de EEE, es DEUC, …) per the
   `.claude/memory/landing-page-design.md` table.
3. Render the `time` eyebrow in `agents-section/index.tsx` — a `font-mono`,
   `text-brand-100`/`brand-200` label above each card's title (near the icon), so
   the triptych reads as a timeline. Keep icons (search/document/trophy), the
   parallel 3-up grid, the brand-700 dark band, and the existing `Reveal` stagger.
4. Extend `landing-content-keys.test.ts` to assert `landing.agents.items[].time`
   is present and non-empty in all 24 locales (alongside the existing
   title/body/lead assertions).
5. Focused tests (vitest binary) + Biome on touched files. **No commit.**

Acceptance: agents section opens on the open-loop headline; each card carries a
`time` eyebrow forming a 02:14 → 05:30 → 07:00 timeline; the wedge lives in the
lead line; all 24 locales carry the `time` field; completeness test green; Biome
clean; no fabricated numbers.

### Secondary flag — search-dock overlaps the audience heading (designer/UX call)

Verified against the code (not fixed): `search-dock/index.tsx` is a
`fixed inset-x-0 bottom-5 z-40 … justify-center` floating pill that only fades via
`useHideNearFooter` (an `IntersectionObserver` on `#site-footer`). So between the
agents band and the footer it stays pinned to the viewport bottom while content
scrolls behind it — **expected floating-dock behavior, not a broken z-index.** The
collision the user saw is *aggravated* because the audience section heading
(`audience-section`, `text-center`) sits on the **same horizontal centre axis** as
the centered dock, so they overlap prominently right as that heading crosses the
bottom band. Options (a designer/UX call — `/ux` or frontend-design, do not fix
blind): accept it as teaser behavior; dim/hide the dock while a centered section
heading is in the bottom band; fade it over more than just the footer; or nudge
its offset. Recommend routing to `neuro-ux-designer` if it bothers the user —
out of scope for this copy/structure pass.

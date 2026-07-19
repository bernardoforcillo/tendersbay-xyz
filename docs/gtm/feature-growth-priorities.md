# Feature growth priorities — which features move the needle

Living doc. Owner: growth. Scope: **which product features to prioritize for
growth**, ranked through the lens of the funnel stage each one serves
(acquisition · activation · retention · referral · pre-launch momentum), with
the behavioral rationale per choice. It does **not** restate positioning or
tone — those live in `.claude/memory/landing-page-design.md`; it maps growth
mechanics onto that positioning.

This is a **prioritization brief, not a feature dump**: a short list of
high-leverage candidates, not everything that could be built. Effort/impact is
qualitative (no invented numbers, pre-launch). Every measurement note is a
funnel sketch to be instrumented later per the `add-posthog-metrics` skill —
consent-safe, no PII in event properties.

---

## The one fact that reframes everything: the CTA is a dead end

The landing's closing CTA already says **"Join the waitlist — Claim your spot"**
(`landing.cta`, all 24 locales) and the body promises market-by-market
activation ("the moment your country lights up, your agents start hunting").
But the page is informational only — **there is no waitlist to join** (per the
landing-page-design memory: "no pricing, no forms/waitlist, no social").

Behaviorally this is the worst possible state: the copy builds desire to a peak
and then offers no release. **Peak-end** says the last thing the visitor feels
is the shape they remember — right now that last thing is a promise with no
door. Every acquisition euro spent driving traffic here leaks out the bottom.

So the growth question "which features are useful?" has a forced first answer:
**the features that turn the existing promise into a captured, compounding
audience.** Everything else is downstream of that.

Note on what already exists in copy: the app locale carries `account`, `auth`,
`tenders`, `today`, `workspace` (with `invites`/`createInvite`/`manageInvites`),
and `workbench` namespaces. So an authenticated product (tender feed, a daily
surface, a bid workbench, team workspaces with invitations) is already being
scaffolded. The features below are framed to plug growth mechanics into that
surface, not to invent a parallel one.

---

## Stage 1 — Pre-launch momentum (this is the current stage; build here first)

### 1.1 Waitlist capture with position-in-line + country gating

**What (concrete):** the "Claim your spot" CTA opens a minimal capture — email +
country (prefilled from locale) + optional persona (run the bids / own the
number / multiply across clients, the three cards that already exist). On submit,
show the person **their position in line for their country** ("You're #142 in
Italy — Italy isn't live yet; you'll be first in when it lights up").

**Behavioral rationale:**
- **Loss aversion + endowed progress** — a position number is a possession. Once
  you hold #142 you have something to lose by leaving, and something to improve.
  A bare "thanks, we'll email you" endows nothing.
- **Scarcity that is literally true** — the market-by-market rollout is real, so
  "your country isn't live yet" is honest urgency, not a countdown gimmick. It
  also makes the country field meaningful rather than decorative.
- **Processing fluency** — three fields, agent does the rest; the setup mirrors
  the product promise (you bring intent, agents do the work).
- **Peak-end repair** — gives the emotional peak a release, and makes the last
  beat "I have a place in line," which is the feeling we want remembered.

**Effort / impact:** medium effort (form + consent-safe storage + a
confirmation email). **Highest impact of anything in this doc** — it is the
foundation every other loop attaches to. Without it, referral and country-launch
triggers have nothing to reference.

**Measurement (funnel sketch):**
`landing_view → cta_click → waitlist_signup`. Event `waitlist_signup` with
categorical, non-PII properties only: `country`, `persona`, `locale`. Email is
PII — store it server-side, **never** as an event property. Success metric:
signup rate per landing view, segmented by locale (tells us which of the 24
markets to prioritize activating).

### 1.2 Referral-to-skip-the-line loop

**What (concrete):** every waitlisted person gets a personal invite link. Each
person who joins through it moves the referrer **up their country's queue** (and,
if you want a harder reward, unlocks earlier access when the market opens). A
simple "invites moved you from #142 to #96" state.

**Behavioral rationale:**
- **Reciprocity + goal-gradient** — the closer you are to the front, the harder
  you push to close the gap (goal-gradient: motivation rises as the goal nears).
  Skipping the line is a reward you can *see* moving.
- **Network compounding over paid one-shots** — this is the K-factor engine from
  the launch playbook: it turns each signup into a distribution node instead of a
  dead end. One convinced SME brings the peers in their sector/region.
- **Real scarcity backs the reward** — limited early-access slots per market make
  "skip the line" credible; the reward is genuine, not invented.

**Effort / impact:** medium-high (link generation, attribution, queue re-rank).
High, **compounding** impact — but it is strictly downstream of 1.1, so it is the
*second* build, not the first.

**Measurement (funnel sketch, ship with the loop):**
`invite_link_created → invite_link_visit → waitlist_signup{referred:true}`.
Derive **K-factor** = (invites sent per user) × (invite→signup conversion).
Track queue-jump events to confirm the reward is understood (if people create
links but never share, the reward framing is wrong). Attribution via an opaque
referral code in the URL, resolved server-side — no PII in the analytics event.

### 1.3 Country-launch trigger ("notify me when my market goes live")

**What (concrete):** the mechanism that pays off 1.1's country field. When a
market is switched on, everyone waitlisted for that country gets the "your
country just lit up" email — the exact moment the CTA copy already promises.
Wire the coverage marquee (already built: 27 EU flags with an AVAILABLE toggle)
so a live country visibly flips state on the page too.

**Behavioral rationale:**
- **Anticipation + literally-true urgency** — a dated, personal "your market is
  live now" beats any generic launch blast; it fires when the recipient's own
  scarcity resolves.
- **Fluency + consistency** — it closes the loop the landing opened ("market by
  market", "when your country lights up"), so the product does exactly what the
  copy said. Consistency builds trust cheaply.
- Turns an existing **decorative** asset (the coverage flags) into a conversion
  and re-engagement surface.

**Effort / impact:** low-medium if folded into 1.1 (country is already a field;
this is the send + the marquee state flip). High re-engagement impact — it is the
single highest-intent email tendersbay will ever send a waitlister.

**Measurement:** `country_activated → launch_email_sent → activation_signin`
(post-launch join rate per activated market). This is the pre-launch→launch
handoff metric.

---

## Stage 2 — Acquisition (feed the top of the waitlist)

### 2.1 Free "awarded near you" teaser — real missed tenders, no login

**What (concrete):** a genuinely useful, no-login surface that shows a small set
of **real, recently awarded** tenders for a sector/region ("these were awarded
in your sector last month"). It is the honest, data-backed version of the
currently-disabled search dock — not the full product, a taste of it.

**Behavioral rationale:**
- **Loss aversion, made concrete** — the sharpest frame for this product is that
  SMEs are already losing tenders they never saw. An abstract pitch can't do
  that; *a real award with a real value they didn't bid on* does. Losses loom
  larger than gains — show the loss.
- **Processing fluency** — one concrete example outperforms any amount of
  benefit copy.
- Doubles as an **SEO acquisition surface** (real sector/region data pages =
  Cluster B in the keyword maps), which the pre-launch landing cannot supply.

**Constraint / effort:** requires **real tender data** and award notices. The
search dock is deliberately disabled pre-launch precisely because there's no
data yet, so this is a **"when data is ready"** play, not a day-one build. Do
**not** ship empty doorway pages (the keyword map is explicit about this).
Medium-high effort, high acquisition + SEO impact once data exists.

**Measurement:** `sample_tender_view{sector, region} → cta_click → waitlist_signup`.
Segment by sector/region to learn which verticals convert — feeds which markets
and sectors to prioritize.

### 2.2 Localized guides / blog (already promised in the footer)

**What (concrete):** the two evergreen posts the Italian keyword map already
scoped — "Come compilare il DGUE (e perché non dovresti farlo a mano)" and
"Partecipare a una gara d'appalto senza ufficio gare" — then their equivalents
in the other head-term markets, each with an in-content waitlist CTA.

**Behavioral rationale:**
- **Authority + processing fluency** — answering the exact how-to a bidder is
  stuck on earns trust before any ask; captures Cluster C/D search intent that
  products don't currently own.
- Cutting toward the rigged system ("you shouldn't have to do this by hand"),
  warm toward the reader — on-brand, and it lands on the find→prepare→win story.

**Effort / impact:** low-medium (content, no app code). High long-tail organic
impact; **lowest-risk acquisition channel available pre-launch** because it needs
no product data — only the keyword maps that already exist.

**Measurement:** `blog_view → in_content_cta_click → waitlist_signup`, per post
and per locale, to see which topics/markets pull.

---

## Stage 3 — Activation (design now, matters at launch)

### 3.1 Tender-profile onboarding → first real match in the first session

**What (concrete):** first-run flow captures the bid profile (sector / CPV,
region, company size, maybe past awards) and — the point — **surfaces a real
matching tender inside the first session.** The aha is *seeing a tender you'd
actually bid on*, fast.

**Behavioral rationale:**
- **Time-to-value / peak-end** — activation is won or lost on how quickly the
  first real match appears. The first session's peak should be a match, not a
  settings screen.
- **Endowed progress** — a filled profile is a sunk investment that pulls the
  user back; the agents "working on your behalf" framing rewards completion.
- **Fluency** — few fields, agents infer the rest (mirrors the product promise).

**Effort / impact:** medium (post-launch, needs the matching engine). High —
it's the gate between signup and habit; a weak first-match kills retention
before it starts.

**Measurement (the core activation funnel):**
`signin → profile_completed → first_match_viewed` **within the first session**.
That last transition is the activation metric to optimize above all others.

### 3.2 "Today" daily matched digest (the `today` namespace already exists)

**What (concrete):** a daily surface + email of newly matched tenders — the habit
trigger. This is the incumbents' entire product (keyword/region → daily alert),
so it is **table stakes**; tendersbay's edge is that the matches are cross-border
and come pre-prepared, not just links.

**Behavioral rationale:**
- **Habit loop** — external trigger (daily send) → reward (new matched tenders) →
  investment (save/track one). This is what converts an activated user into a
  retained one.
- Meeting the category's baseline expectation (alerts) is a **fluency** move:
  don't make switchers relearn the core value.

**Effort / impact:** medium. High retention impact; but it depends on 3.1 (no
profile, no matches to send).

**Measurement:** `digest_sent → digest_open → tender_opened → tender_saved`.
Retention cohort: share of activated users still opening the digest at D7/D30.

---

## Stage 4 — Retention (the open-loop that pulls users back)

### 4.1 Deadline tracking + bid pipeline (the `workbench` namespace)

**What (concrete):** track tenders you're preparing, with **real deadline
reminders** and a simple pipeline (watching → preparing → submitted → outcome).

**Behavioral rationale:**
- **Literally-true urgency** — tender deadlines are real dates; reminders are
  honest urgency, the fair kind the hard rules allow.
- **Zeigarnik + commitment/consistency** — an open, unfinished bid is a
  psychological open loop that pulls you back; having started a submission, you
  return to finish it. A bid in progress is the single strongest reason to come
  back.

**Effort / impact:** medium-high (post-launch). High retention impact — a bid in
the pipeline is the best predictor a user will return.

**Measurement:** `deadline_reminder_sent → return_session`; bids-in-pipeline as a
leading retention indicator (does pipeline count predict D30 retention?).

### 4.2 Outcome tracking ("did you win?") — and the proof it eventually buys

**What (concrete):** after a deadline passes, ask the outcome (awarded / not /
withdrew). Low-friction, one tap.

**Behavioral rationale:**
- Closes the find→prepare→**win** narrative in the product itself (peak-end on
  the whole journey).
- Compounds into the **only legitimate social proof this product can ever
  have**: real, consented, aggregated outcomes. Pre-launch there are zero
  invented testimonials or success rates (hard rule); this is the pipe that,
  months post-launch, produces *real* ones. Build the pipe early even though the
  proof arrives late.

**Effort / impact:** low-medium. Medium near-term, high long-term (proof asset).

**Measurement:** `outcome_recorded{result}` — never attach identity in the event;
aggregate only, and gate any external use behind explicit consent.

---

## Stage 5 — Referral / multiplier (post-launch, the highest-leverage node)

### 5.1 Advisor / partner mode — one consultant, a portfolio of SMEs

**What (concrete):** let consultants and accountants (the "multiply across
clients" persona that already has its own landing card) manage **multiple client
workspaces** from one seat, with the per-client data isolation that the assurance
section already promises. The `workspace` + `invites` copy is already scaffolded.

**Behavioral rationale:**
- **Multiplier network over one-shot acquisition** — this is the single
  highest-leverage distribution node in the whole playbook. One convinced advisor
  brings their entire book of SMEs; you acquire a portfolio per conversion, not a
  user.
- **Authority transfer** — the SME trusts their accountant; the accountant's
  adoption is social proof tendersbay doesn't have to manufacture.
- Per-client isolation (already a stated assurance) is the **objection-remover**
  that makes an advisor willing to put clients on it.

**Effort / impact:** medium-high (multi-workspace management, billing later).
**Highest long-term acquisition leverage** — prioritize the advisor persona as a
distribution channel, not just a user segment.

**Measurement:** `workspaces_per_advisor`, `client_invited → client_activated`;
advisor-sourced signups as a share of total. This is the metric that tells you
whether the multiplier is real.

### 5.2 Workspace team invites (already in copy)

**What (concrete):** invite colleagues into a bid workspace (the existing
`invites` / `createInvite` / `manageInvites` copy).

**Behavioral rationale:** collaboration is retention (more seats = more switching
cost) and mild in-org virality. Lower growth leverage than 5.1 (it spreads inside
one company, not across companies), but it's near-free since the copy exists —
ship it with the workbench, don't prioritize it as a growth lever on its own.

**Measurement:** `workspace_invite_sent → invite_accepted → member_activated`.

---

## Recommendation — the top 3 to build first, and why

Given the **pre-launch, market-by-market** stage, prioritize the features that
convert the existing promise into a compounding audience, in this order:

1. **Waitlist capture with position-in-line + country gating (1.1 + 1.3).**
   Non-negotiable and first. The primary CTA currently dead-ends; every other
   growth mechanic references the waitlist, and no acquisition spend is worth
   anything until the bottom of the funnel holds water. Loss aversion + real
   scarcity + peak-end repair, all at once.

2. **Referral-to-skip-the-line loop (1.2).** Immediately after capture, so
   pre-launch traffic compounds into a network instead of accumulating as a flat
   list. This is the K-factor engine; it is what makes the launch a network
   event rather than a paid blast. Ship its funnel definition with it.

3. **Localized guides / blog (2.2).** The only high-quality acquisition channel
   that needs **no product data**, so it can run in parallel while the app is
   built. It feeds the top of the waitlist using keyword maps that already exist,
   at low risk and no app-code dependency.

Deliberately **not** in the first three: the free "awarded near you" teaser (2.1)
and all Stage 3–5 features are gated on real tender data or the authenticated
app, which don't exist yet pre-launch. They are the **next** wave — design the
activation funnel (3.1) now so it's ready the day data lands, but don't build
ahead of the data. Advisor mode (5.1) is the highest *long-term* leverage and
should be the headline of the *post-launch* growth phase.

---

## Implementation handoff (for gtm-engineer — not executed in this run)

Strategy-only ask; nothing built. When the waitlist wave is greenlit, hand these
to `gtm-engineer` (copy, events, flags are its remit — not mine):

- [ ] **Waitlist capture (1.1):** build the "Claim your spot" capture behind the
      existing `landing.cta` button — fields email + country (prefill from active
      locale) + persona (reuse the 3 `landing.audience.items` values). Persist
      email server-side; return the person's per-country queue position. Author
      all supporting copy (confirmation, position state, "your market isn't live
      yet") across all **24 locales** with a completeness test, per the standard
      recipe.
- [ ] **Position-in-line state (1.1):** render the position number and country
      status on submit; wire the coverage marquee's AVAILABLE toggle to reflect a
      live vs. waitlisted country (1.3).
- [ ] **PostHog funnel (1.1), define before instrumenting:** `landing_view →
      cta_click → waitlist_signup`; event props `country`, `persona`, `locale`
      only — **no email or any PII** in event properties (consent-safe per the
      `add-posthog-metrics` skill). Segment signup rate by locale.
- [ ] **Referral loop (1.2):** per-user opaque referral code in the invite URL,
      resolved server-side; queue re-rank on referred signup; "you moved from #X
      to #Y" state. Ship the funnel with it: `invite_link_created →
      invite_link_visit → waitlist_signup{referred:true}`, plus a derived
      K-factor metric. Attribution resolved server-side, no PII in events.
- [ ] **Country-launch trigger (1.3):** on market activation, send the "your
      country lit up" email to that country's waitlist and flip the marquee state.
      Event `country_activated → launch_email_sent → activation_signin`.
- [ ] **Guides/blog (2.2):** stand up the footer-promised blog; first two posts =
      the DGUE and "senza ufficio gare" briefs already scoped in
      `italian-keyword-map.md` §4, each with an in-content waitlist CTA; funnel
      `blog_view → in_content_cta_click → waitlist_signup`.
- [ ] **GDPR:** the capture form needs consent-safe handling of email (PII);
      confirmation/launch emails need a lawful basis and unsubscribe; no PII ever
      in PostHog properties. Confirm against the existing `consent` namespace.

Two upstream questions for the user before the engineer starts, both product
decisions I can't make alone:
- Does a backend endpoint / storage for waitlist emails exist yet, or does this
  need `services/backend` work? (Affects whether this is a gtm-engineer-only task
  or needs a backend hand.)
- What is the real early-access reward for referrals (queue jump only, or genuine
  earlier product access per market)? The reward must be literally true — I need
  the real mechanism before the loop copy can promise it.

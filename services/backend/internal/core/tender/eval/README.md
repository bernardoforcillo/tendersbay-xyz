# Offline relevance harness

This package measures search relevance against a fixed, committed corpus
snapshot, so a ranking change can be judged by a numeric diff instead of by
eyeballing search results. The ledger and per-task reports that produced this
harness live under `.superpowers/` in the worktree that built it — that
directory is gitignored (`.gitignore`) and is deleted once the worktree's
review is clean, so this file is the durable record of what that process
found operationally load-bearing. It is not a replay of that history; see
`hybrid.go`'s `DefaultRanking` comment and this package's own tests for the
full numbers.

## Running it

Three environment variables point the harness at its own, dedicated stack —
never the shared dev database or the shared dev Qdrant collection:

| Variable | Points at |
| --- | --- |
| `EVAL_DATABASE_URL` | An evaluation-only Postgres database (e.g. `tendersbay_eval`), migrated by `services/ingestion`'s migrations. `newHarness` in `harness_test.go` connects directly with no migration step of its own. |
| `EVAL_QDRANT_URL` | A Qdrant instance **dedicated** to this eval corpus — see the warning below, this is not optional. |
| `EVAL_OLLAMA_URL` | The Ollama server used to embed queries at search time (and tenders at load time). |

`EVAL_EMBEDDING_MODEL` is optional (defaults to `embeddinggemma:latest` in
both `harness_test.go` and `cmd/eval-load`) and only needs setting if the
corpus was embedded with a different model.

Once the stack is up (see below) and the eval database/Qdrant collection are
populated (also below), run the gate:

```sh
cd services/backend
EVAL_DATABASE_URL="postgres://root:toor@localhost:5432/tendersbay_eval?sslmode=disable" \
EVAL_QDRANT_URL="http://localhost:6533" \
EVAL_OLLAMA_URL="http://localhost:11434" \
go test ./internal/core/tender/eval/ -run TestHarness -v
```

Every environment variable is required — `newHarness`'s `requireEnv` calls
`t.Skip` (not `t.Fatal`) when any is unset, so the suite SKIPs cleanly with no
env vars set at all (the CI behaviour: no eval stack exists there, and a
skipped gate is honest, whereas a stack-shaped failure someone learns to
ignore is worse than no gate).

## The dedicated-Qdrant-instance requirement

`EVAL_QDRANT_URL` must point at a Qdrant instance **dedicated** to this eval
corpus, not merely a different URL on the shared dev instance. The reason is
structural, not a convention: `go-services/knowledge/qdrant.go` hard-codes

```go
collectionName = "tenders"
```

as a package constant, so nothing this harness does can select a different
*collection* on a *shared* instance — the only real isolation is a separate
instance. `cmd/eval-load`'s package doc explains the consequence in detail:
the eval database's `bigserial` id sequence starts fresh at 1, exactly like a
real tender's, and Qdrant documents are keyed by that decimal id — so on a
shared instance, the eval loader's ids and real ingestion's ids would
eventually collide and silently overwrite each other's embeddings.

Bring up the dedicated instance once (this repo's convention: dev Qdrant on
the default `6333`, eval Qdrant on `6533`):

```sh
podman run -d --name tendersbay-eval-qdrant -p 6533:6333 qdrant/qdrant:latest
```

`EVAL_QDRANT_URL=http://localhost:6533` then points at it.

## Populating the stack: eval-export → eval-load → harness

Three separate steps, only the last two of which are the normal "I have a
laptop rebuild / fresh eval stack, get me back to a running harness" path:

1. **`cmd/eval-export`** (rare — only when the committed snapshot itself needs
   regenerating) reads a **populated** database via the plain `DATABASE_URL`
   env var and writes the fixed corpus snapshot committed at
   `testdata/corpus.jsonl.gz`:

   ```sh
   DATABASE_URL="postgres://root:toor@localhost:5432/tendersbay?sslmode=disable" \
   go run ./cmd/eval-export -out internal/core/tender/eval/testdata/corpus.jsonl.gz -limit 4000
   ```

   Do not run this casually: every recorded baseline is only comparable
   within one snapshot, so regenerating it invalidates
   `testdata/baseline.json` and every number in `hybrid.go`'s history
   comments. `-limit` must stay well above `(distinct source countries) *
   -per-country` (default 120) or the balance-per-country windowing silently
   truncates the alphabet's tail (see the flag's own `-help` text) —
   `-limit 2000` reproduces exactly that truncation bug; `4000` is what was
   actually used for the current committed snapshot.

2. **`cmd/eval-load`** is the normal entry point for standing the harness back
   up: it reads the committed snapshot and populates BOTH the target
   `EVAL_DATABASE_URL` database and the dedicated `EVAL_QDRANT_URL` Qdrant
   collection, keyed by the eval database's own row id (required — see the
   command's package doc for why `source:source_ref` keying does not work
   with the current search path).

   ```sh
   EVAL_DATABASE_URL="postgres://root:toor@localhost:5432/tendersbay_eval?sslmode=disable" \
   EVAL_QDRANT_URL="http://localhost:6533" \
   EVAL_OLLAMA_URL="http://localhost:11434" \
   go run ./cmd/eval-load
   ```

   **This costs roughly 75 minutes for the committed ~3,000-tender corpus** —
   embedding through Ollama runs at roughly 40 tenders/minute. It is
   idempotent (tracked via the eval database's own `indexed_at` column,
   exactly like real ingestion's indexer), so a re-run after an interruption
   only embeds the remainder, and a re-run against an already-fully-loaded
   database costs seconds, not minutes. `-force-embed` re-embeds everything
   regardless of `indexed_at`, for the rare case where the embedding model
   itself changed underneath an otherwise-loaded corpus.

3. Run the harness (see "Running it" above).

## `EVAL_WRITE_BASELINE=1`

Set on `TestHarness_NoRegressionAgainstBaseline` to **record** the current
scores into `testdata/baseline.json` instead of comparing against it:

```sh
EVAL_WRITE_BASELINE=1 EVAL_DATABASE_URL=... EVAL_QDRANT_URL=... EVAL_OLLAMA_URL=... \
go test ./internal/core/tender/eval/ -run TestHarness_NoRegressionAgainstBaseline -v
```

Use this **only** deliberately, when a ranking change genuinely moved the
scores, and commit the updated `testdata/baseline.json` in the **same commit**
as the change that moved it — never as a separate "oh, the gate failed, let me
just rewrite the baseline" commit. That discipline is the entire point of the
gate: every relevance change becomes a reviewable numeric diff instead of a
silent drift. Never set it just to make a failing gate pass.

## Corpus limitation: no descriptions

**0 of the 3,030 snapshot tenders carry a `description`** (verified directly
by loading `testdata/corpus.jsonl.gz` and checking every row, not merely
asserted from the export). All 3,030 rows in this particular snapshot are
`source="ted"` — the ingested TED rows had no description populated at export
time. Titles only, averaging ~147 characters; TED's title format is
"Country – Category – Subject", so the CPV category name IS present in the
title, in the notice's own language — which is part of why lexical retrieval
still works reasonably well on this corpus despite the gap.

Consequences, all downstream of the same fact:

- `search_vector`'s weight class D (`description`) is empty for every row in
  this corpus. Every measured lexical score in `testdata/baseline.json`
  reflects a title/buyer/cpv-only vector, not the full four-class one
  production actually builds once a tender has a description.
- The dense arm's embedding text is built from title+buyer only, for the same
  reason.
- `ts_headline` snippets fall back to title (the SQL already `coalesce`s
  `NULLIF(description, '')` to title for exactly this reason).
- The CPV bridge itself (see below) is unaffected by this gap — it keys on
  CPV codes, not description text — but the harness measures it in a
  **thinner** setting than production, with less competing text signal, which
  biases the harness slightly in the CPV arm's favour relative to a
  description-rich corpus.

**Every baseline recorded in this file moves if descriptions are ever
ingested into the source database and the snapshot is re-exported.** That is
expected and not itself a regression — but it does mean a future re-export
needs a fresh `EVAL_WRITE_BASELINE=1` run in the same commit, and the
resulting diff should be read with this limitation in mind, not compared
naively against the numbers in `hybrid.go`'s history comments.

## What was measured about the CPV bridge

The whole point of this harness was to measure a cross-language CPV bridge —
resolving free text onto CPV codes (identical in all 24 EU languages) so a
query in one language can find notices written in another. Two independent
mechanisms were built and measured, and **both ship disabled**:

- **The retrieval arm** (`Ranking.CPVWeight`, `cpv.go`'s `FindByCPVPrefixes`):
  fetches tenders whose CPV matches a resolved code as a fourth ranked list,
  fused by RRF. Measured at multiple prefix-truncation depths, with its own
  ordering and candidate-window bugs found and fixed along the way — every
  configuration scored below the two-arm (lexical+dense) baseline, because
  RRF fuses on rank and rank carries no relevance information *within* one
  CPV category. Ships permanently off (`CPVWeight: 0`); the code is kept
  because the ordering/window fixes underneath it are real and a future
  re-attempt would want to start from a correct baseline instead of
  rediscovering the same bugs.
- **Index-side label expansion** (formerly `Ranking.CPVIndexExpanded` /
  `LexicalQuery.ExpandCPVLabels`, migration 0008's `cpv_labels` column):
  denormalised each tender's CPV labels, in all 24 languages, into
  `search_vector` itself, so a query could match a label lexically. Measured
  across all four `{CPVWeight, CPVIndexExpanded}` combinations: it moved
  **none** of the four target cross-language cells off `0.0000` and cost real
  ndcg/mrr on the same-language diagonal, because `websearch_to_tsquery` ANDs
  every query word against an unstemmed, undiacritic-folded official taxonomy
  vocabulary that natural queries simply don't use the same words as. This
  mechanism was not just shipped disabled but **removed entirely** in the
  final whole-branch review: its shipped-off path narrowed the match with
  `ts_filter(search_vector, …)`, a function call that is not the indexed
  expression, which silently turned every lexical search into a sequential
  table scan in production (measured: 88ms → ~3s on the live 13k-row dev
  table). Migration 0009 reverted `search_vector` to its pre-0008 shape; the
  `cpv_labels` **column** stays (cheap, populated, and useful to a future
  attempt at a different match strategy — per-word OR, stemming, or a
  synonym bridge), it is just no longer part of the indexed vector.

The one CPV signal that *does* ship enabled is `Ranking.CPVBoost` (1.15): a
post-fusion multiplier on candidates lexical/dense already found, rather than
a separate retrieval or an index change. It is a real but modest gain — see
`hybrid.go`'s `DefaultRanking` comment for the full numbers and for why the
gain sits partly below the harness's own regression-tolerance noise floor.

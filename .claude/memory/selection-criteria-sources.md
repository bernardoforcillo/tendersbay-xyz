---
name: selection-criteria-sources
description: "What each tender source actually publishes as selection criteria — TED eForms is a pointer at the ESPD, ES PLACSP is inline prose with real thresholds; why category and type are two columns and why there is no weight or threshold column"
metadata:
  type: project
  updated: 2026-08-22
  sources: [docs/superpowers/prd/2026-08-17-competitive-program.md]
---

Selection criteria are the *admissibility* conditions a buyer publishes (turnover floors,
technical capacity, declarations) — distinct from **award** criteria, which are scored.
Sources differ so sharply on what they publish that the difference decides which markets can
have machine-comparable eligibility at all. How a published criterion is then modelled
downstream is [[published-requirement-modelling]].

## TED / eForms: a pointer, not data

Where the eForms qualification block appears, its content is a **signpost to a document we
do not have**. In the real notice fixture
`services/ingestion/internal/adapter/source/eforms/testdata/notice_548186-2026_single_lot_weighted.xml`,
all four `cac:TendererQualificationRequest` blocks are exhausted by:

- `selection-criteria-source = epo-sub-espd`
- `exclusion-grounds-source = epo-sub-espd`
- `reserved-procurement = res-ws`
- `CompanyLegalFormCode listName="required"` = `false`

`epo-sub-espd` means *"the criteria are in the ESPD document."* No BT-749 name, no amount, no
category. So parsing eForms qualification for Italy yields a pointer, not a requirement — the
eForms adapter reads none of it today, and reading it would gain almost nothing.

**Sampling caveat, which must not be dropped when this is quoted.** n = 2 fixtures — the real
notice above and `derived_multilot_distinct_values.xml` — and they were chosen by the parser
authors for their *weighting and multi-lot* properties, not sampled for qualification
content. This therefore establishes a **structural** fact (where the `epo-sub-espd` idiom is
used, there is nothing to parse) and **not a frequency**. "What share of Italian notices set
`selection-criteria-source = epo-sub-espd` versus carrying real criteria" is still an open
measurement, and the way to answer it is the **stored corpus** (`xml_fetched_at` rows in
Postgres, `xpath()` or a plain `LIKE '%epo-sub-espd%'`), not the network and not a text
search tool — see [[positive-control-for-negative-results]] for the two measurement methods
that already failed here and must not be repeated.

## ES / PLACSP: Spain publishes what TED points at

PLACSP embeds `cac:TendererQualificationRequest` **inline in its ATOM search feed**, as prose
carrying real thresholds and the legal basis — e.g. *"un volumen anual de negocios […] igual
o superior a 309.552,00 €, equivalente a una vez y media el VEC (artículo 87.3.a) LCSP)"*.
Three CODICE families (`services/ingestion/.../source/es/codice/parse.go`):

| Family | Domain `Category` |
| --- | --- |
| `TechnicalEvaluationCriteria` | `technical` |
| `FinancialEvaluationCriteria` | `financial` |
| `SpecificTendererRequirement` | `declaration` |

- **`Category` and `Type` are two columns because the code alone is ambiguous.** PLACSP emits
  the bare code `"5"` under a financial list and `"1"` under a declaration list; without the
  family they are integers that collide. `Type` stays verbatim (CODICE `OSR-TECH`,
  `OSR-COMPTASK`, `5`, `1`) — mapping it onto a domain vocabulary is the reader's job, and
  this layer records what was published in the form it was published in.
- **`LotRef` is left empty for ES**: CODICE publishes the qualification block once per
  contract folder, never per lot. Writing a lot ref we don't have would invent a scope the
  source never expressed. Lot-scoped entries stay in one flat list keyed by `LotRef` rather
  than split onto `Lot` the way award criteria are — that split exists so stored aggregates
  can be cross-checked against child rows, and selection criteria have no such aggregate.

## No weight column, no threshold column — deliberately

`tenders.ingested_tender_selection_criteria` (ingestion migration `0012_selection_criteria`)
carries neither, and the migration says why at length:

- **Weight**, because selection is pass/fail. A column NULL on every row ever written is not
  a nullable column, it is a mistake with a default.
- **Threshold**, because *no observed source publishes a comparable one.* Spain states it in
  prose; TED points at the ESPD. A column NULL on every row reads, to anyone writing a query,
  as though populated meant "we checked" — the same error a `docs_coverage` enum was rejected
  for one table over.

## Wiring notes worth reusing

- `DetailedSource` is an **optional companion interface**, not a widened `Source`, so the four
  existing providers compile untouched; `runSource` type-asserts and falls back to plain
  `Fetch`. A provider with nothing extra to give shouldn't have to say so.
- Criteria are keyed by **`SourceRef`, not tender ID** — IDs are the sink's business and don't
  exist yet at fetch time. The ES test asserts the *relationship*
  (`map key == tenders[0].SourceRef`) rather than a literal, because if the two ever drift the
  criteria are silently dropped and a literal assertion would not notice.
- A document publishing **no** criteria contributes **no map entry**, not an empty one. Under
  replace semantics an empty set is not "nothing to say" — it is an instruction to clear the
  rows — so the distinction is load-bearing and has its own test.
- `SaveSelectionCriteria` is deliberately **not** routed through `SaveDetail`, even though
  `NoticeDetail` is where these live in the model: `SaveDetail`'s last statement stamps
  `xml_status = ok` / `xml_fetched_at = now()`, recording that the notice document was *read*.
  PLACSP tenders have no notice document, so going through that door would take the row out of
  the enrichment queue by claiming a read that never happened.

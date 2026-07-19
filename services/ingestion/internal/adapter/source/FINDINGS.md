# National source spike — endpoints, field mappings, caveats (2026-07-19)

Real samples captured live into each `<cc>/testdata/`. Network is reachable from this env (no WAF
block; all endpoints returned 200). **These fixtures are the source of truth — reconcile struct tags
to them.**

## 🇵🇱 Poland — BZP (`pl-bzp`)

- **Endpoint (platform `bzpapi`):** `GET https://ezamowienia.gov.pl/mo-board/api/v1/Board/Search?pageNumber=1&pageSize=N`
  → `application/json`, **a JSON array** of notice objects. No OAuth for reads. (The
  `mo-client-board/...` path is the SPA shell — do NOT use it. `mo-board/api/v1/notice` is a
  different, param-strict endpoint returning `problem+json` — not this one.)
- **Fixture:** `pl/testdata/notice_sample.json` (5 notices).
- **Mapping (`bzp.Map`):**
  | tender.Tender | BZP field |
  | --- | --- |
  | SourceRef | `objectId` (stable GUID; `bzpNumber` is the human "2026/BZP …" number — keep in Raw) |
  | Title | `orderObject` |
  | Buyer.Name | `organizationName` |
  | CPV | `cpvCode` |
  | Deadline | `submittingOffersDate` |
  | PublishedAt | `publicationDate` |
  | Status | from `noticeType`: `ContractNotice`→open; `ContractAwardNotice`→awarded; `ContractPerformingNotice`/other→unknown (verify names in fixture) |
  | Value | **nil** — the search API exposes only `isTenderAmountBelowEU` (bool); keep the bool in `Raw` for Spec 2 |
  | Country/Language | `"PL"` / `"pl"` |
  | Documents | `pdfUrl` → `Document{URL, Type:"notice"}` if non-empty |

## 🇫🇷 France — BOAMP (`fr-boamp`)

- **Endpoint (platform `boampapi`):** `GET https://boamp-datadila.opendatasoft.com/api/records/1.0/search/?dataset=boamp&rows=N&sort=dateparution`
  → `application/json`, `{"nhits":…, "records":[{"recordid":…, "fields":{…}}]}`. Paginate with `start`.
- **Fixture:** `fr/testdata/record_sample.json` (2 records).
- **Mapping (`boamp.Map`), fields under `records[].fields`:**
  | tender.Tender | BOAMP field |
  | --- | --- |
  | SourceRef | `idweb` |
  | Title | `objet` |
  | Buyer.Name | `nomacheteur` |
  | Deadline | `datelimitereponse` |
  | PublishedAt | `dateparution` |
  | Status | from `nature`/`nature_categorise`: contains "attribution"→awarded; "marché/marche" or `nature_categorise` avis-de-marché→open; else unknown |
  | Country/Language | `"FR"` / `"fr"` |
- **⚠ CAVEAT — CPV:** the flat `fields.descripteur_code` ("475","344") is BOAMP's own *descripteur*
  taxonomy, **not** an 8-digit CPV. Real CPV lives in the nested `fields.donnees` full-notice blob.
  The `boamp` parser must dig CPV out of `donnees` (parse it) — or, if not cleanly available, leave
  `CPV=""` and note it (the qualification agent degrades gracefully). Decide from the fixture.

## 🇪🇸 Spain — PLACSP (`es-placsp`)

- **Endpoint (platform `placspapi`):** `GET https://contrataciondelsectorpublico.gob.es/sindicacion/sindicacion_643/licitacionesPerfilesContratanteCompleto3.atom`
  → ATOM `<feed>`; **CODICE is INLINE in each `<entry>`** (namespaces `cac:`/`cbc:`/`cbc-place:`).
  Paginate via `<link rel="next" href="…"/>` at the feed level.
- **Fixture:** `es/testdata/atom_sample.xml` — the full 1.47 MB feed. **First implementation step:
  trim it to ~2 `<entry>` elements** (keep the `<feed>` wrapper + namespaces + a synthesized
  `<link rel="next">`) for a fast, focused test fixture; keep the trimmed file committed.
- **CODICE paths (`codice.Parse`, `cac:`/`cbc:`/`cbc-place:` namespaces — confirmed present):**
  | Document field | CODICE path |
  | --- | --- |
  | ContractFolderID | `cac:ProcurementProjectLot`… root `cbc:ContractFolderID` (48 present) |
  | Title | `cac:ProcurementProject/cbc:Name` |
  | CPV | `cac:RequiredCommodityClassification/cbc:ItemClassificationCode` (169 present; may repeat → primary + secondary) |
  | EstimatedValue | `cac:ProcurementProject/cac:BudgetAmount/cbc:TotalAmount` or `cbc:TaxExclusiveAmount` → **minor units** |
  | Currency | the amount's `currencyID` attribute |
  | Deadline | `cac:TenderingProcess/cac:TenderSubmissionDeadlinePeriod/cbc:EndDate` (+`cbc:EndTime`) (45 present) |
  | Status | `cbc-place:ContractFolderStatusCode` — sample value `EV`; map `PUB`/`EV`→open, `ADJ`/`RES`→awarded, `ANUL`→cancelled, else unknown |
  | NUTS | `cac:RealizedLocation/cbc:CountrySubentityCode` |
  | BuyerName | `cac:LocatedContractingParty/cac:Party/cac:PartyName/cbc:Name` |
- **ES carries a numeric value** (unlike BZP) → `Value` is populated for `es-placsp`.

## Cross-cutting
- All three `Source.Name()`: `pl-bzp`, `fr-boamp`, `es-placsp`.
- No live network in unit tests — everything runs off these `testdata/` fixtures via `httptest`.

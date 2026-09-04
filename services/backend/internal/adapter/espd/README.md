# ESPD / DGUE adapters

Three adapters turn a composed `espd.Response` into a file:

| Package | Produces | Notes |
| --- | --- | --- |
| `edm21` | `QualificationApplicationResponse`, ESPD-EDM **2.1.1** | The model the Italian eDGUE-IT specification binds to |
| `edm4`  | `QualificationApplicationResponse`, ESPD-EDM **4.1.0** | The eForms-era line |
| `pdf`   | A printable DGUE | The artefact a legal representative signs |

`edmxml` holds the shared document builder; `edm21` and `edm4` are its two
profiles. `codelist` holds the vendored criterion tables.

## What "valid" means here — honestly

Three levels of checking exist, and only two of them run in CI:

1. **Golden files** (`edmxml/testdata`) — change detection. The document a
   customer submits must not change as a side effect of an unrelated edit.
   Regenerate deliberately with `go test ./internal/adapter/espd/edmxml -update`
   and review the diff. The vendored criterion definitions are collapsed to a
   marker in the golden so the diff shows what we author.
2. **Structural assertions** — the output parses, carries the version's own
   header, and carries every criterion the operator answered, keyed by the
   official UUID and answered against the official property UUID.
3. **Official Schematron validation** — *not in CI*. The ESPD-EDM release ships
   Schematron rules as XSLT, and there is no pure-Go XSLT 2.0 processor;
   `CGO_ENABLED=0` plus `distroless/static` rules out libxml2. Validate a sample
   by hand when the serializers change:

   ```sh
   git clone https://github.com/OP-TED/espd-edm /tmp/espd-edm
   cd /tmp/espd-edm && git checkout v2.1.1
   # Saxon-HE, or any XSLT 2.0 processor:
   java -jar saxon.jar -s:response.xml \
     -xsl:docs/src/main/asciidoc/modules/ROOT/dist/val/schematron/ESPDResponse-2.1.1/xsl/*.xsl
   ```

## Known limits of the XML

The serializers emit, per criterion, the criterion's own answer — the
"Your answer?" indicator — plus the Art. 57(6) self-cleaning pair when a Part
III ground applies. They do **not** fill a criterion's detail properties: the
year and amount under a turnover criterion, the description under a reference.

That is a deliberate stop, not an oversight. Those slots exist and the generator
could hand us their UUIDs, but deciding which value belongs in which slot by
shape is a guess, and a guess in this document is a false declaration carrying
someone's signature. Filling them needs a per-criterion mapping reviewed against
the specification — named work, not a heuristic.

Until then: **the PDF is the complete document** and carries every value; the
XML is the criterion-level declaration, useful for re-import and for portals
that accept it. Present it that way in the UI — "for reuse and import where
supported", never "this is what you submit".

Part III.D is a related simplification: the ESPD has one yes/no for the whole
national catalogue (`CRITERION.EXCLUSION.NATIONAL.OTHER`), so every national
ground the operator declared collapses onto it, true when any applies. The PDF
prints them one by one.

## Determinism

Every generated identifier derives from the document's own content, so
re-exporting an unchanged response produces byte-identical bytes. That is what
makes `bid_espd_exports.content_sha256` worth storing: equal hashes mean equal
documents, and a different hash means something really changed.

## The vendored code lists

`codelist/edm211.xml`, `codelist/edm4.xml` and `codelist/tables_gen.go` are
generated from the official releases — © European Union, EUPL-1.2, see
`codelist/NOTICE`. Regenerate them against a newer release with:

```sh
git clone https://github.com/OP-TED/espd-edm /tmp/espd-edm
go run ./internal/adapter/espd/codelist/generate -edm /tmp/espd-edm -out internal/adapter/espd/codelist
```

The generator reads the releases' own sample responses, which carry the complete
criterion definitions including the property UUIDs a response validates against.
The one piece of human judgement in it is the `keys` table — which official
criterion each of our keys means — and the four mappings that are judgement
calls rather than identities are documented there.

## Why the PDF writer is hand-rolled

`github.com/go-pdf/fpdf`, the obvious dependency, last shipped in September
2023. What this package needs is one column of text with headings, wrapped
paragraphs and page breaks, which needs no font embedding and no compression —
a few hundred lines against a permanent unmaintained dependency in a distroless
binary. The Helvetica advance widths in `pdf/metrics.go` are Adobe's published
AFM metrics, and they are what makes the wrapping correct rather than
approximate.

The output is WinAnsi-encoded, which covers the Latin-script locales this
product serves. A locale needing Greek or Cyrillic needs an embedded font; until
then the writer substitutes a visible `?` rather than dropping characters
silently.

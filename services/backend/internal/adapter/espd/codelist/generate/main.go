// Command generate builds the vendored ESPD criterion tables this service
// serializes with, from a local checkout of the official ESPD-EDM repository.
//
// Usage:
//
//	git clone https://github.com/OP-TED/espd-edm /tmp/espd-edm
//	go run ./internal/adapter/espd/codelist/generate -edm /tmp/espd-edm -out internal/adapter/espd/codelist
//
// It reads the OFFICIAL sample response of each release (tags v2.1.1 and
// v4.1.0), because those samples carry the complete criterion definitions —
// criterion UUID, taxonomy code, name, description, legislation and the whole
// property tree with the property UUIDs a response has to answer against.
// Re-deriving that tree by hand is exactly the kind of transcription this
// product refuses to do with a company's facts, and there is no reason to do it
// with the EU's either.
//
// The generator checks out the two tags itself, so one invocation produces both
// tables from one checkout.
//
// Source: https://github.com/OP-TED/espd-edm, licensed under the EUPL-1.2.
// The extracted criterion subtrees are redistributed here under that licence;
// see the NOTICE beside the generated files.
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// release names one ESPD-EDM version and where its sample response lives inside
// that release's tree.
type release struct {
	Tag        string
	SamplePath string
	Out        string // basename, without extension
	Version    string // our espd.Version constant, for the generated comment
}

var releases = []release{
	{
		Tag:        "v2.1.1",
		SamplePath: "docs/src/main/asciidoc/modules/ROOT/dist/xml/EXTENDED-ESPDResponse-2.1.1.xml",
		Out:        "edm211",
		Version:    "EDM211",
	},
	{
		Tag:        "v4.1.0",
		SamplePath: "xml-examples/ESPD-Response.xml",
		Out:        "edm4",
		Version:    "EDM4",
	},
}

func main() {
	edm := flag.String("edm", "", "path to a checkout of https://github.com/OP-TED/espd-edm")
	out := flag.String("out", ".", "directory to write the generated files into")
	flag.Parse()
	if *edm == "" {
		fail(fmt.Errorf("-edm is required"))
	}

	tables := map[string]map[string]criterion{}
	for _, r := range releases {
		if err := checkout(*edm, r.Tag); err != nil {
			fail(fmt.Errorf("checkout %s: %w", r.Tag, err))
		}
		found, err := extract(filepath.Join(*edm, r.SamplePath))
		if err != nil {
			fail(fmt.Errorf("%s: %w", r.Tag, err))
		}
		tables[r.Out] = found
		if err := writeXML(filepath.Join(*out, r.Out+".xml"), r, found); err != nil {
			fail(err)
		}
		fmt.Fprintf(os.Stderr, "%s: %d criteria extracted\n", r.Tag, len(found))
	}
	if err := writeGo(filepath.Join(*out, "tables_gen.go"), tables); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "generate:", err)
	os.Exit(1)
}

func checkout(dir, tag string) error {
	cmd := exec.Command("git", "-C", dir, "checkout", "--quiet", tag)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// criterion is one extracted definition.
type criterion struct {
	UUID     string
	TypeCode string
	Name     string
	// XML is the verbatim <cac:TenderingCriterion> element. A response that
	// omitted the definitions would not round-trip through the official tools,
	// which read the property tree out of the document itself.
	XML string
	// AnswerPropertyID is the property this criterion's answer validates: the
	// FIRST question in document order. For every exclusion ground and almost
	// every selection criterion that is the "Your answer?" INDICATOR the
	// criterion hangs off; for LOTS_TENDERED, which asks for a lot id and not a
	// yes/no, it is a LOT_IDENTIFIER. AnswerDataType records which, so the
	// serializer emits ResponseIndicator or ResponseID rather than assuming.
	AnswerPropertyID string
	AnswerDataType   string
	// SelfCleaningIndicatorID / SelfCleaningTextID are the Art. 57(6) pair —
	// "have you taken measures to demonstrate your reliability" and "please
	// describe them" — present on exclusion grounds only.
	SelfCleaningIndicatorID string
	SelfCleaningTextID      string
}

// node is the minimal shape of the sample document we walk.
type node struct {
	XMLName  xml.Name
	Content  []byte `xml:",innerxml"`
	Children []node `xml:",any"`
	Text     string `xml:",chardata"`
}

func extract(path string) (map[string]criterion, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc node
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := map[string]criterion{}
	var walk func(n node)
	walk = func(n node) {
		if n.XMLName.Local == "TenderingCriterion" {
			c := parseCriterion(n)
			if c.UUID != "" {
				out[c.UUID] = c
			}
			return
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	walk(doc)
	return out, nil
}

func parseCriterion(n node) criterion {
	c := criterion{
		UUID:     child(n, "ID"),
		TypeCode: child(n, "CriterionTypeCode"),
		Name:     child(n, "Name"),
		XML:      "<cac:TenderingCriterion>" + string(n.Content) + "</cac:TenderingCriterion>",
	}
	// The property tree, walked in document order: the first question is the
	// criterion's own answer; a LATER indicator whose group holds a DESCRIPTION
	// is the self-cleaning pair.
	questions := questionProps(n)
	if len(questions) == 0 {
		return c
	}
	c.AnswerPropertyID = questions[0].id
	c.AnswerDataType = questions[0].dataType
	for _, q := range questions[1:] {
		if q.dataType == "INDICATOR" && strings.Contains(strings.ToLower(q.description), "measures to demonstrate") {
			c.SelfCleaningIndicatorID = q.id
			c.SelfCleaningTextID = q.firstDescriptionInSubgroup
			break
		}
	}
	return c
}

type question struct {
	id                         string
	dataType                   string
	description                string
	firstDescriptionInSubgroup string
}

// questionProps returns every QUESTION property in the criterion, in document
// order, each carrying the first DESCRIPTION property of the group nested under
// it (which is what an indicator's "please describe" answer validates).
func questionProps(n node) []question {
	var out []question
	var walkGroup func(g node)
	walkGroup = func(g node) {
		for _, ch := range g.Children {
			switch ch.XMLName.Local {
			case "TenderingCriterionProperty":
				if child(ch, "TypeCode") == "QUESTION" {
					out = append(out, question{
						id:                         child(ch, "ID"),
						dataType:                   child(ch, "ValueDataTypeCode"),
						description:                child(ch, "Description"),
						firstDescriptionInSubgroup: firstDescription(g),
					})
				}
			case "TenderingCriterionPropertyGroup", "SubsidiaryTenderingCriterionPropertyGroup":
				walkGroup(ch)
			default:
				walkGroup(ch)
			}
		}
	}
	walkGroup(n)
	return out
}

// firstDescription finds the first DESCRIPTION question anywhere below g. For a
// self-cleaning indicator that is "Please describe them".
func firstDescription(g node) string {
	for _, ch := range g.Children {
		if ch.XMLName.Local == "TenderingCriterionProperty" &&
			child(ch, "ValueDataTypeCode") == "DESCRIPTION" && child(ch, "TypeCode") == "QUESTION" {
			return child(ch, "ID")
		}
		if id := firstDescription(ch); id != "" {
			return id
		}
	}
	return ""
}

func child(n node, name string) string {
	for _, ch := range n.Children {
		if ch.XMLName.Local == name {
			return strings.TrimSpace(ch.Text)
		}
	}
	return ""
}

// keys is the ONE piece of human judgement in this generator: which official
// criterion each of our CriterionKeys means. It is keyed by criterion UUID
// because the UUIDs are stable across 2.1.1 and 4.1.0 while the type codes are
// not ("CRITERION.EXCLUSION.CONVICTIONS.FRAUD" became "fraud").
//
// Four mappings are judgement calls rather than identities, and each is
// deliberate:
//
//   - iv.a.other_registration -> SUITABILITY.AUTHORISATION. The dossier's
//     "iscrizione" covers white lists, albo gestori ambientali and MEPA, which
//     the ESPD models as an authorisation to pursue the activity.
//   - iv.c.references -> REFERENCES.WORKS_PERFORMANCE. A PastContract carries
//     no works/supplies/services discriminator, and this product's operator is
//     a construction PMI; a buyer asking for supplies references reads the same
//     rows in a different slot, which is a Phase 3 refinement, not a silent
//     mis-statement.
//   - iv.c.soa_attestation -> OTHER.EO_DATA.REGISTERED_IN_OFFICIAL_LIST. That
//     IS what a SOA attestation is in ESPD terms: registration in an official
//     list of approved economic operators.
//   - iii.d.purely_national_grounds -> EXCLUSION.NATIONAL.OTHER, which the ESPD
//     leaves deliberately open for exactly this.
var keys = map[string]string{
	// Part III.A — convictions
	"005eb9ed-1347-4ca3-bb29-9bc0db64e1ab": "iii.a.participation_criminal_organisation",
	"c27b7c4e-c837-4529-b867-ed55ce639db5": "iii.a.corruption",
	"297d2323-3ede-424e-94bc-a91561e6f320": "iii.a.fraud",
	"d486fb70-86b3-4e75-97f2-0d71b5697c7d": "iii.a.terrorist_offences",
	"47112079-6fec-47a3-988f-e561668c3aef": "iii.a.money_laundering",
	"d789d01a-fe03-4ccd-9898-73f9cfa080d1": "iii.a.child_labour_human_trafficking",
	// Part III.B — taxes and social security
	"b61bbeb7-690e-4a40-bc68-d6d4ecfaa3d4": "iii.b.payment_of_taxes",
	"7d85e333-bbab-49c0-be8d-c36d71a72f5e": "iii.b.payment_of_social_security",
	// Part III.C — insolvency, misconduct, conflicts
	"a80ddb62-d25b-4e4e-ae22-3968460dc0a9": "iii.c.breach_environmental_obligations",
	"a261a395-ed17-4939-9c75-b9ff1109ca6e": "iii.c.breach_social_obligations",
	"a34b70d6-c43d-4726-9a88-8e2b438424bf": "iii.c.breach_labour_obligations",
	"d3732c09-7d62-4edc-a172-241da6636e7c": "iii.c.bankruptcy",
	"396f288a-e267-4c20-851a-ed4f7498f137": "iii.c.insolvency",
	"68918c7a-f5bc-4a1a-a62f-ad8983600d48": "iii.c.arrangement_with_creditors",
	"daffa2a9-9f8f-4568-8be8-7b8bf306d096": "iii.c.analogous_situation",
	"8fda202a-0c37-41bb-9d7d-de3f49edbfcb": "iii.c.assets_administered_by_liquidator",
	"166536e2-77f7-455c-b018-70582474e4f6": "iii.c.business_activities_suspended",
	"514d3fde-1e3e-4dcd-b02a-9f984d5bbda3": "iii.c.grave_professional_misconduct",
	"56d13e3d-76e8-4f23-8af6-13e60a2ee356": "iii.c.agreements_distorting_competition",
	"b1b5ac18-f393-4280-9659-1367943c1a2e": "iii.c.conflict_of_interest",
	"61874050-5130-4f1c-a174-720939c7b483": "iii.c.involvement_in_preparation",
	"3293e92b-7f3e-42f1-bee6-a7641bb04251": "iii.c.early_termination",
	"696a75b2-6107-428f-8b74-82affb67e184": "iii.c.misrepresentation",
	// Part III.D
	"63adb07d-db1b-4ef0-a14e-a99785cf8cf6": "iii.d.purely_national_grounds",
	// Part IV.A — suitability
	"6ee55a59-6adb-4c3a-b89f-e62a7ad7be7f": "iv.a.enrolment_professional_register",
	"87b3fa26-3549-4f92-b8e0-3fd8f04bf5c7": "iv.a.enrolment_trade_register",
	"9eeb6d5c-0eb8-48e8-a4c5-5087a7c095a4": "iv.a.other_registration",
	// Part IV.B — economic and financial standing
	"499efc97-2ac1-4af2-9e84-323c2ca67747": "iv.b.general_yearly_turnover",
	"074f6031-55f9-4e99-b9a4-c4363e8bc315": "iv.b.specific_yearly_turnover",
	// Part IV.C — technical and professional ability
	"cdd3bb3e-34a5-43d5-b668-2aab86a73822": "iv.c.references",
	"1f49b3f0-d50f-43f6-8b30-4bafab108b9b": "iv.c.average_annual_manpower",
	"9b19e869-6c89-4cc4-bd6c-ac9ca8602165": "iv.c.soa_attestation",
	// Part IV.D — certificates
	"d726bac9-e153-4e75-bfca-c5385587766d": "iv.d.quality_assurance_certificates",
	"8ed65e48-fd0d-444f-97bd-4f58da632999": "iv.d.environmental_management_certificates",
	// Part II.C / II.D — the operator's choices for this gara
	"0d62c6ed-f074-4fcf-8e9f-f691351d52ad": "ii.c.reliance",
	"72c0c4b1-ca50-4667-9487-461f3eed4ed7": "ii.d.subcontracting",
}

// edm4Renames is where 4.1.0 broke the "identifiers are stable" rule: the
// "other data" criteria lost their UUIDs and took tag-style identifiers. The
// question each asks is the same, so the same key maps to both.
//
// The lots criterion (CRITERION.OTHER.EO_DATA.LOTS_TENDERED, 2.1.1 only) is
// deliberately absent from both tables: lots are expressed as
// cac:ProcurementProjectLot, which both releases carry, and one structural rule
// beats a criterion in one version and an element in the other.
var edm4Renames = map[string]string{
	"C58_OT_registered": "iv.c.soa_attestation",
	"C60_OT_relied":     "ii.c.reliance",
	"C61_OT_subco-ent":  "ii.d.subcontracting",
}

// wantedIn returns the official-identifier -> our-key table for one release.
func wantedIn(tag string) map[string]string {
	out := make(map[string]string, len(keys))
	for id, key := range keys {
		out[id] = key
	}
	if tag != "v4.1.0" {
		return out
	}
	for id, key := range edm4Renames {
		// Drop the 2.1.1 identifier this one replaces, then add the new one.
		for oldID, oldKey := range out {
			if oldKey == key {
				delete(out, oldID)
			}
		}
		out[id] = key
	}
	return out
}

// writeXML writes the criterion definitions we use, in a stable order, as one
// embeddable fragment per release.
func writeXML(path string, r release, found map[string]criterion) error {
	wanted := wantedIn(r.Tag)
	var b strings.Builder
	b.WriteString(xmlHeader(r))
	for _, id := range sortedIDs(wanted) {
		c, ok := found[id]
		if !ok {
			return fmt.Errorf("%s: criterion %s (%s) is absent from the official sample", r.Tag, id, wanted[id])
		}
		if c.AnswerPropertyID == "" {
			return fmt.Errorf("%s: criterion %s (%s) has no question property", r.Tag, id, wanted[id])
		}
		b.WriteString("<!-- " + wanted[id] + " -->\n")
		b.WriteString(c.XML)
		b.WriteString("\n")
	}
	b.WriteString("</criteria>\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func xmlHeader(r release) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!--
  Criterion definitions extracted from the OFFICIAL ESPD-EDM ` + r.Tag + ` sample
  response (` + r.SamplePath + `).

  Source:  https://github.com/OP-TED/espd-edm
  Licence: EUPL-1.2 — © European Union.

  GENERATED by internal/adapter/espd/codelist/generate. Do not edit by hand:
  re-run the generator against a newer release instead.
-->
<criteria xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
`
}

// sortedIDs orders a release's identifiers by OUR key, so the generated files
// have a stable, readable order that does not shuffle when a UUID changes.
func sortedIDs(wanted map[string]string) []string {
	out := make([]string, 0, len(wanted))
	for id := range wanted {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return wanted[out[i]] < wanted[out[j]] })
	return out
}

func writeGo(path string, tables map[string]map[string]criterion) error {
	var b strings.Builder
	b.WriteString(`// Code generated by internal/adapter/espd/codelist/generate. DO NOT EDIT.
//
// Criterion identifiers extracted from the official ESPD-EDM releases
// (https://github.com/OP-TED/espd-edm, EUPL-1.2, © European Union).
//
// The criterion UUIDs are the same in 2.1.1 and 4.1.0; the TYPE CODES are not
// ("CRITERION.EXCLUSION.CONVICTIONS.FRAUD" became "fraud"), and neither are the
// property UUIDs a response validates against. That is precisely why one model
// needs two serializers.

package codelist

import "github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"

`)
	for _, r := range releases {
		found := tables[r.Out]
		wanted := wantedIn(r.Tag)
		fmt.Fprintf(&b, "// %sCriteria is the %s table, keyed by our criterion key.\nvar %sCriteria = map[espd.CriterionKey]Criterion{\n", r.Out, r.Tag, r.Out)
		for _, id := range sortedIDs(wanted) {
			c := found[id]
			fmt.Fprintf(&b, "\t%q: {\n\t\tUUID: %q,\n\t\tTypeCode: %q,\n\t\tName: %q,\n\t\tAnswerPropertyID: %q,\n\t\tAnswerDataType: %q,\n",
				wanted[id], c.UUID, c.TypeCode, c.Name, c.AnswerPropertyID, c.AnswerDataType)
			if c.SelfCleaningIndicatorID != "" {
				fmt.Fprintf(&b, "\t\tSelfCleaningIndicatorID: %q,\n\t\tSelfCleaningTextID: %q,\n", c.SelfCleaningIndicatorID, c.SelfCleaningTextID)
			}
			b.WriteString("\t},\n")
		}
		b.WriteString("}\n\n")
	}
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return fmt.Errorf("gofmt the generated table: %w", err)
	}
	return os.WriteFile(path, src, 0o644)
}

package edmxml_test

import (
	"encoding/xml"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/codelist"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/edm21"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/edm4"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// TestSerializeGolden pins what this package AUTHORS, byte for byte.
//
// The golden files are OURS, not the EU's: the ESPD-EDM release ships sample
// documents, but they describe a different company answering different
// criteria, so they cannot be diffed against our output. What these files are
// for is change detection — a diff here means the document a customer submits
// changed, and that has to be a deliberate act with a reviewed diff, never a
// side effect.
//
// The vendored criterion definitions are collapsed to a one-line marker before
// comparing. Keeping them would make each golden 400 KB of code list we did not
// write and cannot review in a diff, and would hide the fifty lines we did
// write inside it — the opposite of what a golden file is for. Which criteria
// the document carries is still pinned, by the markers.
func TestSerializeGolden(t *testing.T) {
	r := readyResponse(t)
	for _, tc := range []struct {
		name   string
		ser    espd.Serializer
		golden string
	}{
		{"edm211", edm21.New(), "response-edm211.xml"},
		{"edm4", edm4.New(), "response-edm4.xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.ser.Serialize(r)
			if err != nil {
				t.Fatalf("Serialize: %v", err)
			}
			got := redactDefinitions(raw)
			path := filepath.Join("testdata", tc.golden)
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run: go test ./internal/adapter/espd/edmxml -update): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("output differs from %s.\nIf the change is intended, re-run with -update and review the diff.\ngot %d bytes, want %d",
					path, len(got), len(want))
			}
		})
	}
}

// redactDefinitions replaces each vendored <cac:TenderingCriterion> block with
// a marker naming the criterion UUID it carried, so the golden shows WHICH
// criteria are in the document without carrying their full definitions.
func redactDefinitions(doc []byte) []byte {
	const open, close = "<cac:TenderingCriterion>", "</cac:TenderingCriterion>"
	out := string(doc)
	var b strings.Builder
	for {
		i := strings.Index(out, open)
		if i < 0 {
			b.WriteString(out)
			break
		}
		j := strings.Index(out[i:], close)
		if j < 0 {
			b.WriteString(out)
			break
		}
		block := out[i : i+j+len(close)]
		b.WriteString(out[:i])
		b.WriteString(`<cac:TenderingCriterion uuid="` + firstID(block) + `" redacted="vendored definition"/>`)
		out = out[i+j+len(close):]
	}
	return []byte(b.String())
}

// firstID reads the criterion UUID out of a definition block.
func firstID(block string) string {
	i := strings.Index(block, "<cbc:ID")
	if i < 0 {
		return ""
	}
	rest := block[i:]
	open := strings.Index(rest, ">")
	end := strings.Index(rest, "</cbc:ID>")
	if open < 0 || end < 0 || end < open {
		return ""
	}
	return strings.TrimSpace(rest[open+1 : end])
}

// TestSerializeIsDeterministic is the property the export audit rests on: the
// same composed document always produces the same bytes, so a content hash
// that differs really means the document differs.
func TestSerializeIsDeterministic(t *testing.T) {
	r := readyResponse(t)
	for _, ser := range []espd.Serializer{edm21.New(), edm4.New()} {
		first, err := ser.Serialize(r)
		if err != nil {
			t.Fatalf("Serialize: %v", err)
		}
		second, err := ser.Serialize(r)
		if err != nil {
			t.Fatalf("Serialize: %v", err)
		}
		if string(first) != string(second) {
			t.Errorf("%s: two serializations of one response differ", ser.Version())
		}
	}
}

// TestSerializeIsWellFormedAndCarriesTheProfile parses the output back and
// checks the version-specific header, which is the half of the two-serializer
// split that a golden file alone would not explain.
func TestSerializeIsWellFormedAndCarriesTheProfile(t *testing.T) {
	r := readyResponse(t)
	for _, tc := range []struct {
		ser                           espd.Serializer
		ublVersion, versionID         string
		wantsCustomization, wantsPEID bool
		sampleTypeCode                string
	}{
		{edm21.New(), "2.2", "2.1.1", true, false, "CRITERION.EXCLUSION.CONVICTIONS.FRAUD"},
		{edm4.New(), "2.3", "4.1.0", false, true, "fraud"},
	} {
		t.Run(string(tc.ser.Version()), func(t *testing.T) {
			out, err := tc.ser.Serialize(r)
			if err != nil {
				t.Fatalf("Serialize: %v", err)
			}
			var doc struct {
				XMLName            xml.Name
				UBLVersionID       string `xml:"UBLVersionID"`
				VersionID          string `xml:"VersionID"`
				CustomizationID    string `xml:"CustomizationID"`
				ProfileExecutionID string `xml:"ProfileExecutionID"`
				ContractFolderID   string `xml:"ContractFolderID"`
			}
			if err := xml.Unmarshal(out, &doc); err != nil {
				t.Fatalf("output is not well-formed XML: %v", err)
			}
			if doc.XMLName.Local != "QualificationApplicationResponse" {
				t.Errorf("root = %q", doc.XMLName.Local)
			}
			if doc.UBLVersionID != tc.ublVersion || doc.VersionID != tc.versionID {
				t.Errorf("UBL %q / version %q", doc.UBLVersionID, doc.VersionID)
			}
			if (doc.CustomizationID != "") != tc.wantsCustomization {
				t.Errorf("CustomizationID = %q", doc.CustomizationID)
			}
			if (doc.ProfileExecutionID != "") != tc.wantsPEID {
				t.Errorf("ProfileExecutionID = %q", doc.ProfileExecutionID)
			}
			if doc.ContractFolderID != "A0B1C2D3E4" {
				t.Errorf("ContractFolderID = %q, want the procedure reference", doc.ContractFolderID)
			}
			// The type codes are the halves that genuinely differ between the
			// two releases; the criterion UUID does not.
			if !strings.Contains(string(out), ">"+tc.sampleTypeCode+"<") {
				t.Errorf("output does not carry the %s type code for fraud", tc.ser.Version())
			}
			if !strings.Contains(string(out), "297d2323-3ede-424e-94bc-a91561e6f320") {
				t.Error("output does not carry the official fraud criterion UUID")
			}
		})
	}
}

// TestSerializeCarriesEveryAnsweredCriterion: an exported DGUE that silently
// dropped a ground the operator answered would look complete and be false.
func TestSerializeCarriesEveryAnsweredCriterion(t *testing.T) {
	r := readyResponse(t)
	out, err := edm21.New().Serialize(r)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	text := string(out)
	table := codelist.EDM211()
	for _, k := range espd.ExclusionCriteria() {
		// Assert on the criterion UUID from the vendored table rather than on
		// our own key, which never appears in the document: the UUID is what a
		// reader keys off, so this checks the thing that matters and cannot
		// drift from the code list.
		c, err := table.Lookup(k)
		if err != nil {
			t.Fatalf("the code list has no entry for %s: %v", k, err)
		}
		if !strings.Contains(text, c.UUID) {
			t.Errorf("criterion %s (%s) is absent from the document", k, c.UUID)
		}
		if !strings.Contains(text, c.AnswerPropertyID) {
			t.Errorf("criterion %s carries no answer against property %s", k, c.AnswerPropertyID)
		}
	}
	// One response per answered ground, at least.
	if got := strings.Count(text, "<cac:TenderingCriterionResponse>"); got < len(espd.ExclusionCriteria()) {
		t.Errorf("%d responses for %d exclusion grounds", got, len(espd.ExclusionCriteria()))
	}
}

// TestSerializeSelfCleaning: a ground that APPLIES must carry the Art. 57(6)
// measures, and the indicator that says measures were taken.
func TestSerializeSelfCleaning(t *testing.T) {
	d := readyDossier(t)
	for i := range d.Declarations {
		if d.Declarations[i].Criterion == string(espd.CritFraud) {
			d.Declarations[i].Answer = true
			d.Declarations[i].SelfCleaning = "Risarcimento integrale e riorganizzazione del controllo interno."
		}
	}
	in := readyInput(d)
	in.Confirmation.DeclarationsHash = espd.HashDeclarations(d.Declarations)
	r := espd.Compose(d, in, nil, fixedNow)
	if !r.Ready() {
		t.Fatalf("gaps = %+v", r.Gaps)
	}
	out, err := edm21.New().Serialize(r)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if !strings.Contains(string(out), "Risarcimento integrale") {
		t.Error("the self-cleaning description is missing from the document")
	}
}

// TestSerializeRejectsAnUnmappedCriterion pins the loud-failure rule: a
// criterion the version's code list cannot express is an error, never an
// omitted element.
func TestSerializeRejectsAnUnmappedCriterion(t *testing.T) {
	r := readyResponse(t)
	r.Leaves = append(r.Leaves, espd.Leaf{
		Part: espd.PartIVC, Criterion: espd.CriterionKey("iv.c.invented_criterion"),
		Field: "x", Value: espd.TextValue("y"),
		Attribution: company.Attribution{Provenance: company.ProvenanceUserStated},
	})
	if _, err := edm21.New().Serialize(r); err == nil {
		t.Fatal("an unmapped criterion must fail the export")
	} else if !strings.Contains(err.Error(), "iv.c.invented_criterion") {
		t.Errorf("the error must name the criterion: %v", err)
	}
}

package company

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func p[T any](v T) *T { return &v }

func TestProvenanceAuthoritative(t *testing.T) {
	tests := []struct {
		provenance Provenance
		want       bool
	}{
		{ProvenanceUserStated, true},
		{ProvenanceUserConfirmed, true},
		{ProvenanceAgentInferred, false},
		{ProvenanceImported, false},
		{Provenance("nonsense"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.provenance), func(t *testing.T) {
			if got := tt.provenance.Authoritative(); got != tt.want {
				t.Errorf("Authoritative() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvenanceValid(t *testing.T) {
	for _, p := range []Provenance{ProvenanceUserStated, ProvenanceUserConfirmed, ProvenanceAgentInferred, ProvenanceImported} {
		if !p.Valid() {
			t.Errorf("%q should be valid", p)
		}
	}
	if Provenance("").Valid() || Provenance("user-stated").Valid() {
		t.Error("unknown provenance strings must not validate")
	}
}

// TestAttributionValidate pins the rule that keeps a model score off a human
// fact: a confidence on an authoritative provenance is rejected outright rather
// than ignored.
func TestAttributionValidate(t *testing.T) {
	tests := []struct {
		name    string
		attr    Attribution
		wantErr error
	}{
		{
			name: "user stated without confidence",
			attr: Attribution{Provenance: ProvenanceUserStated, PromptedBy: PromptOnboarding},
		},
		{
			name: "agent inferred with confidence",
			attr: Attribution{Provenance: ProvenanceAgentInferred, Confidence: p(0.8)},
		},
		{
			name:    "user stated with confidence",
			attr:    Attribution{Provenance: ProvenanceUserStated, Confidence: p(1.0)},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "user confirmed with confidence",
			attr:    Attribution{Provenance: ProvenanceUserConfirmed, Confidence: p(0.5)},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "confidence out of range",
			attr:    Attribution{Provenance: ProvenanceImported, Confidence: p(1.4)},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "unknown provenance",
			attr:    Attribution{Provenance: "guessed"},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "unknown prompt source",
			attr:    Attribution{Provenance: ProvenanceUserStated, PromptedBy: "vibes"},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "source note too long",
			attr:    Attribution{Provenance: ProvenanceUserStated, SourceNote: strings.Repeat("x", maxSourceNoteLen+1)},
			wantErr: ErrInvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.attr.validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClassificaOrdering(t *testing.T) {
	tests := []struct {
		held, want Classifica
		ok         bool
	}{
		{ClassificaIII, ClassificaII, true},
		{ClassificaII, ClassificaIII, false},
		{ClassificaIII, ClassificaIII, true},
		{ClassificaIIIBis, ClassificaIII, true},
		{ClassificaIII, ClassificaIIIBis, false},
		{ClassificaIVBis, ClassificaIV, true},
		{ClassificaVIII, ClassificaI, true},
	}
	for _, tt := range tests {
		t.Run(tt.held.String()+"_covers_"+tt.want.String(), func(t *testing.T) {
			if got := tt.held.AtLeast(tt.want); got != tt.ok {
				t.Errorf("%s.AtLeast(%s) = %v, want %v", tt.held, tt.want, got, tt.ok)
			}
		})
	}
}

func TestClassificaString(t *testing.T) {
	tests := map[Classifica]string{
		ClassificaI:      "I",
		ClassificaIII:    "III",
		ClassificaIIIBis: "III-bis",
		ClassificaIVBis:  "IV-bis",
		ClassificaVIII:   "VIII",
	}
	for c, want := range tests {
		if got := c.String(); got != want {
			t.Errorf("Classifica(%d).String() = %q, want %q", int(c), got, want)
		}
	}
	// An out-of-range value must be visible, not silently empty.
	if got := Classifica(99).String(); got == "" {
		t.Error("an invalid classifica must render as something")
	}
}

func TestClassificaCeilingEUR(t *testing.T) {
	// The one figure the remedy sentence quotes: OG1 III covers works up to
	// 1.033.000 EUR.
	if got := ClassificaIII.CeilingEUR(); got == nil || *got != 1_033_000_00 {
		t.Fatalf("ClassificaIII.CeilingEUR() = %v, want 103300000", got)
	}
	// VIII is unbounded and must say so with nil, not with a large number that
	// a comparison would silently treat as a real ceiling.
	if got := ClassificaVIII.CeilingEUR(); got != nil {
		t.Fatalf("ClassificaVIII.CeilingEUR() = %v, want nil (unbounded)", *got)
	}
	// Ceilings must increase monotonically with the tier, or AtLeast and the
	// ceiling would disagree about which attestation is bigger.
	prev := int64(0)
	for c := ClassificaI; c <= ClassificaVII; c++ {
		v := c.CeilingEUR()
		if v == nil {
			t.Fatalf("classifica %s has no ceiling", c)
		}
		if *v <= prev {
			t.Fatalf("ceiling for %s (%d) does not exceed the previous tier (%d)", c, *v, prev)
		}
		prev = *v
	}
}

func TestParseClassifica(t *testing.T) {
	tests := []struct {
		in   string
		want Classifica
		ok   bool
	}{
		{"I", ClassificaI, true},
		{"iii", ClassificaIII, true},
		{"III-bis", ClassificaIIIBis, true},
		{"III bis", ClassificaIIIBis, true},
		{"IIIBIS", ClassificaIIIBis, true},
		{"  VIII  ", ClassificaVIII, true},
		{"IX", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParseClassifica(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Errorf("ParseClassifica(%q) = %v,%v want %v,%v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNormalizeSOACategory(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"OG1", "OG1", true},
		{"og 1", "OG1", true},
		{"OG13", "OG13", true},
		{"OS35", "OS35", true},
		{"OG14", "", false},
		{"OS36", "", false},
		{"OG.1", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := NormalizeSOACategory(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Errorf("NormalizeSOACategory(%q) = %q,%v want %q,%v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// TestSOACategoryLapsedAt is the invariant that stops the most likely wrong
// "go": the triennial verification lapses three years before the attestation
// and invalidates it on its own.
func TestSOACategoryLapsedAt(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		validUntil    *time.Time
		verifiedUntil *time.Time
		want          bool
	}{
		{"both in the future", p(now.AddDate(3, 0, 0)), p(now.AddDate(1, 0, 0)), false},
		{"attestation lapsed", p(now.AddDate(-1, 0, 0)), p(now.AddDate(1, 0, 0)), true},
		{"verification lapsed, attestation still valid", p(now.AddDate(2, 0, 0)), p(now.AddDate(-1, 0, 0)), true},
		{"no dates at all", nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := SOACategory{Category: "OG1", Classifica: ClassificaIII, ValidUntil: tt.validUntil, VerifiedUntil: tt.verifiedUntil}
			if got := c.LapsedAt(now); got != tt.want {
				t.Errorf("LapsedAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSOACategoryValidate(t *testing.T) {
	ok := Attribution{Provenance: ProvenanceUserStated}
	tests := []struct {
		name    string
		in      SOACategory
		wantErr error
	}{
		{"valid", SOACategory{Category: "OG1", Classifica: ClassificaIII, Attribution: ok}, nil},
		{"bad category", SOACategory{Category: "OG99", Classifica: ClassificaIII, Attribution: ok}, ErrInvalidSOACategory},
		{"bad classifica", SOACategory{Category: "OG1", Classifica: Classifica(42), Attribution: ok}, ErrInvalidClassifica},
		{"zero classifica", SOACategory{Category: "OG1", Attribution: ok}, ErrInvalidClassifica},
		{"bad attribution", SOACategory{Category: "OG1", Classifica: ClassificaI}, ErrInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("validate() = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestFinancialYearValidate(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	ok := Attribution{Provenance: ProvenanceUserStated}
	tests := []struct {
		name    string
		in      FinancialYear
		wantErr bool
	}{
		{"valid", FinancialYear{Year: 2025, TurnoverMinor: p(int64(100)), Currency: "EUR", Attribution: ok}, false},
		{"future year", FinancialYear{Year: 2030, Attribution: ok}, true},
		{"bad currency", FinancialYear{Year: 2025, Currency: "euro", Attribution: ok}, true},
		{"negative turnover", FinancialYear{Year: 2025, TurnoverMinor: p(int64(-1)), Attribution: ok}, true},
		// Fatturato specifico is a subset of fatturato globale by definition;
		// stating otherwise would satisfy a threshold the company does not meet.
		{"specific exceeds global", FinancialYear{Year: 2025, TurnoverMinor: p(int64(100)), SpecificTurnoverMinor: p(int64(200)), Attribution: ok}, true},
		{"negative headcount", FinancialYear{Year: 2025, Headcount: p(int32(-3)), Attribution: ok}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.validate(now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPastContractEffectiveValueMinor covers the classic way an SME's dossier
// overstates its own track record: counting the whole RTI contract instead of
// its own share.
func TestPastContractEffectiveValueMinor(t *testing.T) {
	tests := []struct {
		name string
		in   PastContract
		want *int64
	}{
		{
			name: "principal counts in full",
			in:   PastContract{Role: RolePrincipal, ValueMinor: p(int64(5_000_000_00))},
			want: p(int64(5_000_000_00)),
		},
		{
			name: "20% mandante counts a fifth",
			in:   PastContract{Role: RoleRTIMandante, ValueMinor: p(int64(5_000_000_00)), SharePct: p(20.0)},
			want: p(int64(1_000_000_00)),
		},
		{
			name: "shared role with no share counts for nothing",
			in:   PastContract{Role: RoleRTIMandataria, ValueMinor: p(int64(5_000_000_00))},
			want: nil,
		},
		{
			name: "no value at all",
			in:   PastContract{Role: RolePrincipal},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.EffectiveValueMinor()
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("EffectiveValueMinor() = %d, want nil", *got)
			case tt.want != nil && got == nil:
				t.Fatalf("EffectiveValueMinor() = nil, want %d", *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Fatalf("EffectiveValueMinor() = %d, want %d", *got, *tt.want)
			}
		})
	}
}

func TestIdentityValidate(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		in      Identity
		wantErr error
	}{
		{"empty is valid", Identity{}, nil},
		{"full", Identity{LegalForm: LegalFormSRL, Country: "IT", NUTS: "ITC4C", FoundedYear: p(int32(1998))}, nil},
		{"unknown legal form", Identity{LegalForm: "spa_srl"}, ErrInvalidArgument},
		{"lowercase country", Identity{Country: "it"}, ErrInvalidArgument},
		{"future founded year", Identity{FoundedYear: p(int32(2030))}, ErrInvalidArgument},
		{
			name:    "unknown attribution key",
			in:      Identity{Attribution: map[FieldKey]Attribution{"partita_iva": {Provenance: ProvenanceUserStated}}},
			wantErr: ErrUnknownFieldKey,
		},
		{
			name:    "invalid attribution under a known key",
			in:      Identity{Attribution: map[FieldKey]Attribution{FieldVATNumber: {Provenance: ProvenanceUserStated, Confidence: p(0.9)}}},
			wantErr: ErrInvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.validate(now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("validate() = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDossierEmpty(t *testing.T) {
	if !(Dossier{}).Empty() {
		t.Error("a zero dossier is empty")
	}
	// One SOA line answered in chat is not an empty dossier, even with no
	// identity — that is the normal shape under just-in-time capture.
	d := Dossier{SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaI}}}
	if d.Empty() {
		t.Error("a dossier with one captured fact is not empty")
	}
}

func TestFactKind(t *testing.T) {
	tests := []struct {
		name string
		in   Fact
		want FactKind
		ok   bool
	}{
		{"identity", Fact{Identity: &IdentityFact{Key: FieldVATNumber}}, FactIdentity, true},
		{"soa", Fact{SOA: &SOACategory{}}, FactSOA, true},
		{"certification", Fact{Certification: &Certification{}}, FactCertification, true},
		{"financial year", Fact{FinancialYear: &FinancialYear{}}, FactFinancialYear, true},
		{"past contract", Fact{PastContract: &PastContract{}}, FactPastContract, true},
		{"registration", Fact{Registration: &Registration{}}, FactRegistration, true},
		{"empty", Fact{}, "", false},
		// A Fact is the answer to ONE question and its attribution is stamped
		// once, for one datum — two records is an error, not a merge.
		{"two records", Fact{SOA: &SOACategory{}, Registration: &Registration{}}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.in.Kind()
			if ok != tt.ok {
				t.Fatalf("Kind() ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("Kind() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRequirementDedupeKey pins the identities the unique constraint relies on:
// a re-extraction that reads a different classifica must UPDATE the OG1 row, and
// two distinct free-text requirements must not collapse into one.
func TestRequirementDedupeKey(t *testing.T) {
	og1III := Requirement{Kind: RequirementSOA, Text: "SOA OG1 III", SOA: &SOARequirement{Category: "OG1", Classifica: ClassificaIII}}
	og1IV := Requirement{Kind: RequirementSOA, Text: "SOA OG1 IV", SOA: &SOARequirement{Category: "OG1", Classifica: ClassificaIV}}
	os30 := Requirement{Kind: RequirementSOA, Text: "SOA OS30 I", SOA: &SOARequirement{Category: "OS30", Classifica: ClassificaI}}

	if og1III.DedupeKey() != og1IV.DedupeKey() {
		t.Error("a re-read classifica must be a correction of the same requirement, not a second one")
	}
	if og1III.DedupeKey() == os30.DedupeKey() {
		t.Error("two different SOA categories are two different requirements")
	}

	// Case and whitespace in the published category must not fork the key.
	loose := Requirement{Kind: RequirementSOA, Text: "x", SOA: &SOARequirement{Category: "og 1", Classifica: ClassificaIII}}
	if loose.DedupeKey() != og1III.DedupeKey() {
		t.Errorf("normalised category should share a key: %q vs %q", loose.DedupeKey(), og1III.DedupeKey())
	}

	// Certifications key on (standard, scope).
	q1 := Requirement{Kind: RequirementCertification, Text: "ISO 9001", Certification: &CertificationRequirement{Standard: CertISO9001}}
	q2 := Requirement{Kind: RequirementCertification, Text: "ISO 9001 costruzioni", Certification: &CertificationRequirement{Standard: CertISO9001, Scope: "costruzioni"}}
	if q1.DedupeKey() == q2.DedupeKey() {
		t.Error("a scoped certification requirement is a different requirement")
	}

	// Free text falls back to a bounded hash, and distinct texts stay distinct.
	a := Requirement{Kind: RequirementOther, Text: "Iscrizione all'albo dei periti"}
	b := Requirement{Kind: RequirementOther, Text: "Polizza assicurativa RCT/RCO"}
	if a.DedupeKey() == b.DedupeKey() {
		t.Error("two distinct free-text requirements must not collapse")
	}
	if len(a.DedupeKey()) > 128 {
		t.Errorf("dedupe key must stay bounded for the btree unique: %d chars", len(a.DedupeKey()))
	}
	// The same text is the same requirement, whatever the whitespace.
	spaced := Requirement{Kind: RequirementOther, Text: "  Iscrizione   all'albo dei periti "}
	if spaced.DedupeKey() != a.DedupeKey() {
		t.Error("whitespace must not fork a free-text requirement")
	}
}

func TestRequirementCanBlock(t *testing.T) {
	confirmedAt := time.Now()
	tests := []struct {
		name string
		in   Requirement
		want bool
	}{
		{"user stated", Requirement{Source: RequirementUserStated}, true},
		{"document excerpt, confirmed", Requirement{Source: RequirementDocumentExcerpt, ConfirmedBy: "u1", ConfirmedAt: &confirmedAt}, true},
		{"document excerpt, unconfirmed", Requirement{Source: RequirementDocumentExcerpt}, false},
		{"award criteria", Requirement{Source: RequirementAwardCriteria, ConfirmedBy: "u1"}, false},
		{"agent inferred", Requirement{Source: RequirementAgentInferred, ConfirmedBy: "u1"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.CanBlock(); got != tt.want {
				t.Errorf("CanBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequirementValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      Requirement
		wantErr error
	}{
		{
			name: "valid soa",
			in:   Requirement{Kind: RequirementSOA, Source: RequirementUserStated, Text: "SOA OG1 III", SOA: &SOARequirement{Category: "OG1", Classifica: ClassificaIII}},
		},
		{
			name: "valid other with no payload",
			in:   Requirement{Kind: RequirementOther, Source: RequirementDocumentExcerpt, Text: "Polizza RCT"},
		},
		{
			name:    "missing text",
			in:      Requirement{Kind: RequirementOther, Source: RequirementUserStated},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "soa without payload",
			in:      Requirement{Kind: RequirementSOA, Source: RequirementUserStated, Text: "x"},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "soa with bad category",
			in:      Requirement{Kind: RequirementSOA, Source: RequirementUserStated, Text: "x", SOA: &SOARequirement{Category: "ZZ9", Classifica: ClassificaI}},
			wantErr: ErrInvalidSOACategory,
		},
		{
			name:    "unknown kind",
			in:      Requirement{Kind: "vibes", Source: RequirementUserStated, Text: "x"},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "unknown source",
			in:      Requirement{Kind: RequirementOther, Source: "hearsay", Text: "x"},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "turnover without amount",
			in:      Requirement{Kind: RequirementTurnover, Source: RequirementUserStated, Text: "x"},
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "text too long",
			in:      Requirement{Kind: RequirementOther, Source: RequirementUserStated, Text: strings.Repeat("x", maxRequirementTextLen+1)},
			wantErr: ErrInvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

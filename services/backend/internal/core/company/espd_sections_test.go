package company

import (
	"context"
	"errors"
	"testing"
	"time"
)

func humanAttr() Attribution {
	return Attribution{Provenance: ProvenanceUserStated, StatedBy: testUser, StatedAt: evalNow, PromptedBy: PromptOnboarding}
}

// TestDeclarationValidateRefusesNonAuthoritativeProvenance pins the legal wall
// of Part III: a declaration can only be user_stated or user_confirmed. An
// agent-inferred or imported one is refused with ErrInvalidArgument (wrapped),
// never downgraded or stored with a warning.
func TestDeclarationValidateRefusesNonAuthoritativeProvenance(t *testing.T) {
	for _, prov := range []Provenance{ProvenanceAgentInferred, ProvenanceImported} {
		t.Run(string(prov), func(t *testing.T) {
			d := Declaration{Criterion: "iii.a.corruption", Attribution: Attribution{Provenance: prov, Confidence: p(0.9)}}
			err := d.validate()
			if !errors.Is(err, ErrDeclarationNotAuthoritative) || !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("validate() = %v, want ErrDeclarationNotAuthoritative wrapping ErrInvalidArgument", err)
			}
			g := NationalGround{Country: "IT", Criterion: "it.art94", Attribution: Attribution{Provenance: prov, Confidence: p(0.9)}}
			if err := g.validate(); !errors.Is(err, ErrDeclarationNotAuthoritative) {
				t.Fatalf("national ground validate() = %v, want ErrDeclarationNotAuthoritative", err)
			}
		})
	}
	for _, prov := range []Provenance{ProvenanceUserStated, ProvenanceUserConfirmed} {
		d := Declaration{Criterion: "iii.a.corruption", Attribution: Attribution{Provenance: prov, PromptedBy: PromptOnboarding}}
		if err := d.validate(); err != nil {
			t.Errorf("%s: validate() = %v, want nil", prov, err)
		}
	}
}

func TestDeclarationValidateShape(t *testing.T) {
	tests := []struct {
		name string
		d    Declaration
		ok   bool
	}{
		{"missing criterion", Declaration{Attribution: humanAttr()}, false},
		{"self-cleaning on a ground that does not apply", Declaration{Criterion: "iii.a.fraud", Answer: false, SelfCleaning: "we fixed it", Attribution: humanAttr()}, false},
		{"self-cleaning on a ground that applies", Declaration{Criterion: "iii.a.fraud", Answer: true, SelfCleaning: "we fixed it", Attribution: humanAttr()}, true},
		{"plain no", Declaration{Criterion: "iii.a.fraud", Attribution: humanAttr()}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.d.validate()
			if (err == nil) != tt.ok {
				t.Errorf("validate() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestNationalGroundValidateCountry(t *testing.T) {
	g := NationalGround{Country: "Italy", Criterion: "it.art94", Attribution: humanAttr()}
	if err := g.validate(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("validate() = %v, want ErrInvalidArgument for a non alpha-2 country", err)
	}
	g.Country = "IT"
	if err := g.validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
}

func TestRepresentativeValidate(t *testing.T) {
	ok := Representative{Role: "legale_rappresentante", GivenName: "Anna", FamilyName: "Rossi", Attribution: humanAttr()}
	if err := ok.validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
	for name, bad := range map[string]Representative{
		"no role":   {GivenName: "Anna", FamilyName: "Rossi", Attribution: humanAttr()},
		"no name":   {Role: "procuratore", Attribution: humanAttr()},
		"bad email": {Role: "procuratore", GivenName: "A", FamilyName: "B", Email: "not-an-email", Attribution: humanAttr()},
	} {
		if err := bad.validate(); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: validate() = %v, want ErrInvalidArgument", name, err)
		}
	}
	// A representative may be imported (from a visura, say): it is not a
	// declaration, so the provenance wall does not apply to it.
	imported := Representative{Role: "procuratore", GivenName: "A", FamilyName: "B", Attribution: Attribution{Provenance: ProvenanceImported, Confidence: p(0.7)}}
	if err := imported.validate(); err != nil {
		t.Errorf("imported representative: validate() = %v, want nil", err)
	}
}

// TestIsSMEIsAnIdentityFieldWithItsOwnAttribution: false-with-no-entry and
// false-with-an-entry are two different states, and UpdateIdentity must stamp
// the entry only when the answer actually changed.
func TestIsSMEIsAnIdentityFieldWithItsOwnAttribution(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

	id, err := svc.UpdateIdentity(ctx, testUser, testWS, Identity{LegalName: "Acme", IsSME: true}, Attribution{})
	if err != nil {
		t.Fatalf("UpdateIdentity: %v", err)
	}
	if a, ok := id.Attribution[FieldIsSME]; !ok || a.Provenance != ProvenanceUserStated {
		t.Fatalf("is_sme attribution = %+v, %v; want a user_stated entry", a, ok)
	}

	// Re-stating the same identity leaves the stamp untouched.
	later := evalNow.Add(time.Hour)
	svc.now = func() time.Time { return later }
	id, err = svc.UpdateIdentity(ctx, testUser, testWS, Identity{LegalName: "Acme", IsSME: true}, Attribution{})
	if err != nil {
		t.Fatalf("UpdateIdentity: %v", err)
	}
	if got := id.Attribution[FieldIsSME].StatedAt; !got.Equal(evalNow) {
		t.Errorf("unchanged is_sme was re-stamped at %v", got)
	}

	// And the just-in-time capture path parses it as a boolean.
	if err := svc.RecordFact(ctx, testUser, testWS, Fact{Identity: &IdentityFact{Key: FieldIsSME, Value: "false"}}, PromptTender, nil); err != nil {
		t.Fatalf("RecordFact: %v", err)
	}
	if repo.dossier.Identity.IsSME {
		t.Error("RecordFact did not clear is_sme")
	}
	if err := svc.RecordFact(ctx, testUser, testWS, Fact{Identity: &IdentityFact{Key: FieldIsSME, Value: "maybe"}}, PromptTender, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("RecordFact(is_sme=maybe) = %v, want ErrInvalidArgument", err)
	}
}

// TestPutDeclarationStampsAndUpserts: the service stamps user_stated whatever
// the caller sent (a client cannot forge a stronger or weaker claim), and a
// second answer to the same criterion replaces the first.
func TestPutDeclarationStampsAndUpserts(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

	forged := Attribution{Provenance: ProvenanceAgentInferred, Confidence: p(0.2), StatedBy: "someone-else"}
	first, err := svc.PutDeclaration(ctx, testUser, testWS, Declaration{Criterion: "iii.b.taxes", Answer: false, Attribution: forged})
	if err != nil {
		t.Fatalf("PutDeclaration: %v", err)
	}
	if first.Provenance != ProvenanceUserStated || first.StatedBy != testUser || first.Confidence != nil {
		t.Errorf("stamped attribution = %+v, want user_stated by %s with no confidence", first.Attribution, testUser)
	}

	second, err := svc.PutDeclaration(ctx, testUser, testWS, Declaration{Criterion: "iii.b.taxes", Answer: true, SelfCleaning: "rateizzazione concessa"})
	if err != nil {
		t.Fatalf("PutDeclaration (re-answer): %v", err)
	}
	if second.ID != first.ID || len(repo.dossier.Declarations) != 1 || !repo.dossier.Declarations[0].Answer {
		t.Errorf("re-answer did not replace the first: %+v", repo.dossier.Declarations)
	}

	if _, err := svc.PutDeclaration(ctx, testUser, testWS, Declaration{Criterion: "iii.b.taxes", Answer: false, SelfCleaning: "x"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("contradictory declaration accepted: %v", err)
	}
	if _, err := svc.PutDeclaration(ctx, testUser, testWS, Declaration{Criterion: "iii.b.taxes"}); err == nil {
		t.Log("ok")
	}
	viewerSvc := newTestService(repo, &fakeReqRepo{}, viewer(), fakeTenders{}, fakeDocuments{})
	if _, err := viewerSvc.PutDeclaration(ctx, testUser, testWS, Declaration{Criterion: "iii.a.fraud"}); err == nil {
		t.Error("a viewer wrote a declaration")
	}
	if _, err := viewerSvc.PutRepresentative(ctx, testUser, testWS, Representative{Role: "r", GivenName: "a", FamilyName: "b"}); err == nil {
		t.Error("a viewer wrote a representative")
	}
	if _, err := viewerSvc.PutNationalGround(ctx, testUser, testWS, NationalGround{Country: "IT", Criterion: "it.x"}); err == nil {
		t.Error("a viewer wrote a national ground")
	}
}

func TestPutNationalGroundUppercasesCountry(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})
	g, err := svc.PutNationalGround(context.Background(), testUser, testWS, NationalGround{Country: " it ", Criterion: "it.art94.c1"})
	if err != nil {
		t.Fatalf("PutNationalGround: %v", err)
	}
	if g.Country != "IT" || g.Provenance != ProvenanceUserStated {
		t.Errorf("got %+v", g)
	}
}

func TestDossierDeclarationForAndEmpty(t *testing.T) {
	d := Dossier{Declarations: []Declaration{{Criterion: "iii.a.fraud", Answer: true}}}
	if d.Empty() {
		t.Error("a dossier with a declaration is not empty")
	}
	if got, ok := d.DeclarationFor("iii.a.fraud"); !ok || !got.Answer {
		t.Errorf("DeclarationFor = %+v, %v", got, ok)
	}
	if _, ok := d.DeclarationFor("iii.a.corruption"); ok {
		t.Error("unanswered criterion reported as answered")
	}
}

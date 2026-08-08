// Package company — this file renders an Assessment as the ordered go/no-go
// answer the product is allowed to give.
//
// The order IS the content of this file: the blocking FACT first, the verdict
// second, what would change it third. That is not a presentation preference. A
// recommendation that opens with "non partecipare" is a loss-framed instruction
// from a machine, and the person who carries the legal liability for ignoring it
// discounts it; the identical content read as "richiede SOA OG1 III, il profilo
// indica OG1 I — con avvalimento si partecipa" is a fact they can verify and a
// remedy they can act on. Encoding the order HERE, once, is what stops every
// consumer (the agent tool's JSON, the proto message, the eligibility panel)
// from re-deciding it independently and getting it wrong somewhere.
//
// Audience: these strings are prose for the MODEL and for a log line — the same
// audience Gap.Held and CaptureQuestion.Prompt already serve — so they are
// written in the language the gara is written in. The 24-locale UI does not read
// them: it renders Gap/Remedy, which are structured precisely so it can localise
// them itself.
package company

import (
	"fmt"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// Caveat is a machine-readable honesty flag: something true about the EVIDENCE
// that a reader would otherwise have to infer from an absence, and absences are
// exactly what get read as "fine".
//
// Each one exists because the alternative is a silent pass. They are a list and
// not a single "confidence" scalar for the reason the design gives for dropping
// confidence_band from the analytics event: a synthetic band would hide the one
// distinction that decides what to build next — insufficient_data because we
// read nothing, versus because the user has not answered a question.
type Caveat string

const (
	// CaveatNoRequirements — nothing at all has been captured for this tender.
	// The correct answer is "we know nothing about this gara's requisiti", never
	// "no gaps found".
	CaveatNoRequirements Caveat = "no_requirements_captured"
	// CaveatAwardGridOnly — everything captured came from the award grid, which
	// says how an ADMITTED bid is scored and nothing about admission. Presenting
	// it as an eligibility check is a wrong answer, not a partial one.
	CaveatAwardGridOnly Caveat = "award_grid_only"
	// CaveatUnconfirmedExtraction — a requirement was quoted out of a document
	// and no human has checked the quote against the passage.
	CaveatUnconfirmedExtraction Caveat = "unconfirmed_extraction"
	// CaveatOpenQuestions — at least one requirement cannot be checked because
	// the dossier holds no answer. This is the just-in-time capture trigger, and
	// the product's answer to it is a question, not a guess.
	CaveatOpenQuestions Caveat = "open_questions"
	// CaveatDocumentsUnread — we do not hold the text of the tender's own
	// documents, so the requisiti section has not been read by anyone or
	// anything. Structured notice data cannot substitute: the selection criteria
	// live in the disciplinare, and the eForms path that would carry them
	// (cac:TendererQualificationRequest, BT-747/748/749) is parsed nowhere in
	// this repo. Silence about this is how ignorance gets read as a clean bill.
	CaveatDocumentsUnread Caveat = "documents_unread"
)

// Recommendation is one Assessment rendered in reading order.
//
// Verdict sits BETWEEN Lead and Changes deliberately: a consumer that walks the
// struct in declaration order — which the agent tool's JSON and the panel both
// effectively do — cannot reach the verdict before the fact it rests on, and
// cannot stop at the verdict without meeting the remedy.
type Recommendation struct {
	// Lead is the single fact the decision turns on, stated before any verdict:
	// what the gara demands and what the dossier answers. Always set — when
	// there is nothing to lead with, it says so in those words.
	Lead string
	// Verdict repeats Assessment.Verdict so a consumer holding only the
	// recommendation still cannot render it without the Lead above it.
	Verdict Verdict
	// Changes is "what would change it", one line per gap that has a remedy,
	// in the same domain order as Gaps. A blocking gap with no line would read
	// as unsolvable, which for an Italian SOA or turnover gap is usually false —
	// avvalimento and RTI exist precisely for it.
	Changes []string
	// Caveats are the honesty flags above, in a fixed order so two runs over the
	// same assessment produce identical output.
	Caveats []Caveat
}

// maxLeadTextLen bounds a requirement's verbatim text inside a rendered line.
// Requirement.Text is allowed up to 4000 characters (a whole disciplinare
// clause), and these lines land in a model prompt and a log record where an
// unbounded quote costs tokens and readability. The full text is always
// available on the Gap itself.
const maxLeadTextLen = 200

// Recommendation renders a in reading order. It is a pure function of the
// assessment — same input, byte-identical output — because it is asserted by a
// table test and because a recommendation that varied between two renders of the
// same evidence would be indefensible to the person acting on it.
func (a Assessment) Recommendation() Recommendation {
	return Recommendation{
		Lead:    a.lead(),
		Verdict: a.Verdict,
		Changes: a.changes(),
		Caveats: a.caveats(),
	}
}

// lead states the fact the decision turns on. Gaps arrive pre-sorted by the
// domain (blocking, then unknown, then expiring, then other unmet, then met), so
// the leading gap is the one the reader must see first and this function only
// has to say it in words.
func (a Assessment) lead() string {
	if len(a.Gaps) == 0 {
		// The honest majority case, and the one most likely to be quietly lost.
		// "No requirements captured" is NOT "no gaps found": Phase 1a persists
		// AWARD criteria only, and a gara's requisiti di partecipazione live in
		// the disciplinare, which for most Italian tenders we have not read.
		return "Per questa gara non è stato registrato alcun requisito di partecipazione. " +
			"I dati strutturati del bando contengono i criteri di aggiudicazione — come viene " +
			"valutata un'offerta ammessa — non i requisiti di partecipazione, che vivono nel " +
			"disciplinare. Non è quindi possibile alcun giudizio di ammissibilità."
	}

	g := a.Gaps[0]
	text := truncateText(g.Requirement.Text)
	held := strings.TrimSpace(g.Held)
	if held == "" {
		held = "nessun dato"
	}

	switch {
	case g.Blocking:
		return fmt.Sprintf("Richiede: %s. Il profilo aziendale indica: %s.", text, held)
	case g.Status == GapUnknown:
		return fmt.Sprintf("Non sappiamo se il requisito è soddisfatto — %s. "+
			"Il profilo aziendale non contiene questo dato.", text)
	case g.Status == GapExpiring:
		return fmt.Sprintf("Richiede: %s. Il profilo aziendale indica: %s, che scade prima "+
			"del termine di presentazione.", text, held)
	case g.Status != GapMet:
		// An unmet requirement the capture marked non-blocking: a criterio
		// premiante, not a requisito di partecipazione. Saying so is the
		// difference between "you are out" and "you score lower".
		return fmt.Sprintf("Richiede: %s. Il profilo aziendale indica: %s. "+
			"Non è un requisito di partecipazione bloccante.", text, held)
	default:
		return fmt.Sprintf("Tutti i %d requisiti registrati risultano soddisfatti dal profilo aziendale.",
			a.RequirementCount)
	}
}

// changes renders "what would change it", one line per remediable gap.
func (a Assessment) changes() []string {
	out := make([]string, 0, len(a.Gaps))
	for _, g := range a.Gaps {
		if g.Remedy.Kind == RemedyNone {
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s", truncateText(g.Requirement.Text), remedyPhrase(g)))
	}
	return out
}

// remedyPhrase turns a Remedy into the sentence a user can act on, delta
// included. The delta is what makes the line worth reading: "serve una
// classifica più alta" is something they already knew; "serve la classifica III,
// che copre lavori fino a 1033000.00 EUR" is not.
func remedyPhrase(g Gap) string {
	var phrase string
	switch g.Remedy.Kind {
	case RemedyProvideFact:
		phrase = "rispondi alla domanda sul profilo aziendale"
	case RemedyRenew:
		phrase = "rinnova il documento prima del termine di presentazione"
	case RemedyUpgrade:
		phrase = "innalza il requisito posseduto"
	case RemedyAvvalimento:
		phrase = "avvalimento su un altro operatore (art. 104 D.Lgs 36/2023)"
	case RemedyRTI:
		phrase = "partecipa in RTI con un operatore che lo possiede"
	case RemedySubcontract:
		phrase = "copri la categoria scorporabile in subappalto"
	default:
		phrase = string(g.Remedy.Kind)
	}

	if c := g.Remedy.MissingClassifica; c != nil {
		if ceiling := c.CeilingEUR(); ceiling != nil {
			phrase += fmt.Sprintf(" — serve la classifica %s, che copre lavori fino a %s",
				c.String(), renderAmount(*ceiling, "EUR"))
		} else {
			// Classifica VIII is unbounded; saying "fino a <nil>" would be worse
			// than saying nothing, and "illimitata" is the published meaning.
			phrase += fmt.Sprintf(" — serve la classifica %s (illimitata)", c.String())
		}
	}
	if m := g.Remedy.MissingAmountMinor; m != nil {
		phrase += fmt.Sprintf(" — mancano %s", renderAmount(*m, amountCurrency(g.Requirement)))
	}
	if n := g.Remedy.MissingCount; n != nil {
		phrase += fmt.Sprintf(" — mancano %d", *n)
	}
	if g.Remedy.DeadlineRelevant {
		phrase += "; deve essere in essere entro il termine di presentazione"
	}
	return phrase
}

// amountCurrency reads the currency the requirement itself named, so a shortfall
// is quoted in the unit the gara stated rather than in an assumed EUR.
func amountCurrency(r Requirement) string {
	if r.Amount == nil {
		return ""
	}
	return r.Amount.Currency
}

// caveats lists the honesty flags in a FIXED order — the order they should be
// read in, most-fundamental first — so two renders of the same assessment are
// byte-identical.
func (a Assessment) caveats() []Caveat {
	var out []Caveat
	if a.RequirementCount == 0 {
		out = append(out, CaveatNoRequirements)
	}
	if a.AwardGridOnly() {
		out = append(out, CaveatAwardGridOnly)
	}
	if a.HasUnconfirmedExtraction() {
		out = append(out, CaveatUnconfirmedExtraction)
	}
	if a.UnknownGapCount > 0 {
		out = append(out, CaveatOpenQuestions)
	}
	// Coverage below "full" means we hold, at best, the notice PDF — and the
	// requisiti are not in the notice. The zero value of document.Coverage is ""
	// (no availability was established at all), which is also not full, and is
	// also worth saying.
	if a.DocumentCoverage != document.CoverageFull {
		out = append(out, CaveatDocumentsUnread)
	}
	return out
}

// truncateText bounds a verbatim requirement inside a rendered line, cutting on
// a rune boundary so the result stays valid UTF-8 (Italian requisiti are full of
// accented characters, and a byte-sliced "à" would render as a replacement
// character in the model's context).
func truncateText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLeadTextLen {
		return s
	}
	runes := []rune(s)
	limit := maxLeadTextLen
	if len(runes) < limit {
		limit = len(runes)
	}
	return strings.TrimRight(string(runes[:limit]), " ") + "…"
}

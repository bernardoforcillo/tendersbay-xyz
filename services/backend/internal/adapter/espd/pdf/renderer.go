package pdf

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// Renderer draws a composed espd.Response as a printable DGUE. It satisfies
// espd.Renderer.
//
// Unlike the XML serializers, the PDF carries EVERY value the response holds:
// it is the artefact a human signs, so a field it omitted would be a field the
// signatory never saw. It also carries the provenance of the Part III answers —
// who declared them and when — because that is what the person signing is
// actually being asked to stand behind.
type Renderer struct{}

// New builds the renderer.
func New() *Renderer { return &Renderer{} }

var _ espd.Renderer = (*Renderer)(nil)

// Render produces the document.
func (r *Renderer) Render(resp espd.Response, opts espd.RenderOptions) ([]byte, error) {
	l := labelsFor(opts.Locale)
	d := newDoc(l.footer)

	r.header(d, resp, l, opts)
	r.parts(d, resp, l)
	r.closing(d, resp, l)

	return d.render(), nil
}

func (r *Renderer) header(d *doc, resp espd.Response, l labels, opts espd.RenderOptions) {
	d.text(l.title, Bold, 16)
	d.text(l.subtitle, Regular, 9)
	d.space(6)
	d.rule()

	row(d, l.buyer, leafText(resp, espd.CritProcedure, "buyer_name"))
	row(d, l.procedure, leafText(resp, espd.CritProcedure, "title"))
	row(d, l.reference, leafText(resp, espd.CritProcedure, "reference"))
	if notice := leafText(resp, espd.CritProcedure, "notice_ref"); notice != "" {
		row(d, l.notice, notice)
	}
	if lots := allText(resp, espd.CritLots, "lot_ref"); len(lots) > 0 {
		row(d, l.lots, strings.Join(lots, ", "))
	}
	row(d, l.version, string(opts.Version))
	row(d, l.composedAt, resp.ComposedAt.UTC().Format("2006-01-02 15:04 MST"))
	d.space(4)
	d.rule()
}

// parts walks the document in Regulation 2016/7 order. Every part with content
// gets a heading; an empty part is skipped rather than printed empty, because a
// heading over nothing reads as a missing answer.
func (r *Renderer) parts(d *doc, resp espd.Response, l labels) {
	for _, p := range []espd.Part{
		espd.PartIIA, espd.PartIIB, espd.PartIIC, espd.PartIID,
		espd.PartIIIA, espd.PartIIIB, espd.PartIIIC, espd.PartIIID,
		espd.PartIVA, espd.PartIVB, espd.PartIVC, espd.PartIVD,
	} {
		leaves := leavesIn(resp, p)
		if len(leaves) == 0 {
			continue
		}
		d.space(8)
		d.text(l.part(p), Bold, 11)
		d.space(2)

		// Group by criterion, then by the record each leaf came from, so a
		// dossier with three SOA lines prints three blocks and not one
		// interleaved list.
		for _, k := range criteriaIn(leaves) {
			printed := false
			for _, src := range sourcesOf(leaves, k) {
				if espd.IsExclusionCriterion(k) {
					r.declaration(d, resp, k, l)
					printed = true
					break
				}
				if !printed {
					d.wrapped(l.criterion(k), Bold, 9, 0)
					printed = true
				}
				for _, leaf := range leavesOf(leaves, k, src) {
					row(d, "   "+l.field(leaf.Field), leaf.Value.String())
				}
				d.space(2)
			}
		}
	}
}

// declaration prints one Part III answer with its provenance — the line that
// makes the re-confirmation meaningful.
func (r *Renderer) declaration(d *doc, resp espd.Response, k espd.CriterionKey, l labels) {
	for _, a := range resp.Declarations.Answers {
		if a.Criterion != k || !a.Answered {
			continue
		}
		d.wrapped(l.criterion(k), Bold, 9, 0)
		answer := l.no
		if a.Applies {
			answer = l.yes
		}
		row(d, "   "+l.answer, answer)
		if a.Applies && strings.TrimSpace(a.SelfCleaning) != "" {
			d.wrapped(l.selfCleaning+": "+a.SelfCleaning, Regular, 9, 14)
		}
		if !a.Attribution.StatedAt.IsZero() {
			d.wrapped(fmt.Sprintf(l.declaredOn, a.Attribution.StatedAt.UTC().Format("2006-01-02")), Italic, 7.5, 14)
		}
		d.space(2)
		return
	}
}

// closing is Part VI: the statement the signatory signs, and the honest note
// about what this file is and is not.
func (r *Renderer) closing(d *doc, resp espd.Response, l labels) {
	d.space(10)
	d.rule()
	d.text(l.partVI, Bold, 11)
	d.space(2)
	d.wrapped(l.declarationText, Regular, 9, 0)
	d.space(8)

	if c := resp.Declarations.Confirmation; c != nil {
		d.wrapped(fmt.Sprintf(l.confirmedOn, c.ConfirmedAt.UTC().Format("2006-01-02 15:04 MST")), Italic, 8, 0)
	}
	d.space(16)
	d.wrapped(l.signatureLine, Regular, 9, 0)
	d.space(20)
	d.wrapped(l.notSigned, Italic, 8, 0)
}

// ── Layout helpers ──────────────────────────────────────────────────────────

// row prints "label: value" with the value wrapped under a hanging indent.
func row(d *doc, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	d.wrapped(label+": "+value, Regular, 9, 0)
}

func leavesIn(r espd.Response, p espd.Part) []espd.Leaf {
	var out []espd.Leaf
	for _, l := range r.Leaves {
		if l.Part == p {
			out = append(out, l)
		}
	}
	return out
}

// criteriaIn lists the criteria present, in first-appearance order, which is
// the order Compose emitted them and therefore document order.
func criteriaIn(leaves []espd.Leaf) []espd.CriterionKey {
	var out []espd.CriterionKey
	seen := map[espd.CriterionKey]bool{}
	for _, l := range leaves {
		if !seen[l.Criterion] {
			seen[l.Criterion] = true
			out = append(out, l.Criterion)
		}
	}
	return out
}

func sourcesOf(leaves []espd.Leaf, k espd.CriterionKey) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range leaves {
		if l.Criterion == k && !seen[l.Source.ID] {
			seen[l.Source.ID] = true
			out = append(out, l.Source.ID)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return false }) // keep document order
	return out
}

func leavesOf(leaves []espd.Leaf, k espd.CriterionKey, src string) []espd.Leaf {
	var out []espd.Leaf
	for _, l := range leaves {
		if l.Criterion == k && l.Source.ID == src {
			out = append(out, l)
		}
	}
	return out
}

func leafText(r espd.Response, k espd.CriterionKey, field string) string {
	for _, l := range r.Leaves {
		if l.Criterion == k && l.Field == field {
			return l.Value.String()
		}
	}
	return ""
}

func allText(r espd.Response, k espd.CriterionKey, field string) []string {
	var out []string
	for _, l := range r.Leaves {
		if l.Criterion == k && l.Field == field {
			out = append(out, l.Value.String())
		}
	}
	return out
}

// Package pdf renders a composed espd.Response as a printable DGUE.
//
// # Why there is a PDF writer in here at all
//
// The obvious move is a library. The candidate the design named,
// github.com/go-pdf/fpdf, has not shipped a release since September 2023, and
// the alternatives in pure Go are either heavier than the whole rest of this
// adapter or equally stale. What this package needs is narrow — one column of
// text, headings, label/value rows, wrapped paragraphs, page breaks, a footer —
// and a PDF that only draws text with the standard Helvetica faces needs no
// font embedding, no compression and no image support. That is a few hundred
// lines against a permanent dependency in a distroless binary, so it is written
// here, with the AFM widths that make text wrapping correct rather than
// approximate.
//
// The output is a PDF 1.4 document with WinAnsi-encoded text, which covers
// every character the 24 EU locales this product serves need for a Latin-script
// document. A locale that needs Greek or Cyrillic needs an embedded font, and
// this writer says so rather than dropping the characters silently.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// Page geometry, in PDF points (1/72"). A4 with the margins a filed document
// wants.
const (
	pageWidth    = 595.28
	pageHeight   = 841.89
	marginLeft   = 56.7 // 20mm
	marginRight  = 56.7
	marginTop    = 56.7
	marginBottom = 62.0 // a little more: the footer lives here
	contentWidth = pageWidth - marginLeft - marginRight
)

// Font is one of the three standard faces this writer uses.
type Font string

const (
	Regular Font = "F1" // Helvetica
	Bold    Font = "F2" // Helvetica-Bold
	Italic  Font = "F3" // Helvetica-Oblique
)

// doc accumulates page content streams.
type doc struct {
	pages   []*bytes.Buffer
	current *bytes.Buffer
	y       float64 // distance from the top of the page to the next baseline
	footer  string
}

// newDoc starts a document whose every page carries footer at the bottom.
func newDoc(footer string) *doc {
	d := &doc{footer: footer}
	d.newPage()
	return d
}

func (d *doc) newPage() {
	buf := &bytes.Buffer{}
	d.pages = append(d.pages, buf)
	d.current = buf
	d.y = marginTop
}

// space advances the cursor, breaking the page when the next line would fall
// into the footer.
func (d *doc) space(h float64) {
	d.y += h
	if d.y > pageHeight-marginBottom {
		d.newPage()
	}
}

// text draws one line at the current cursor and advances past it.
func (d *doc) text(s string, f Font, size float64) {
	if d.y+size > pageHeight-marginBottom {
		d.newPage()
	}
	fmt.Fprintf(d.current, "BT /%s %.1f Tf 1 0 0 1 %.2f %.2f Tm (%s) Tj ET\n",
		f, size, marginLeft, pageHeight-d.y-size, escape(s))
	d.y += size * 1.35
}

// wrapped draws s across as many lines as it needs, indented by indent points.
func (d *doc) wrapped(s string, f Font, size, indent float64) {
	for _, line := range wrap(s, f, size, contentWidth-indent) {
		if d.y+size > pageHeight-marginBottom {
			d.newPage()
		}
		fmt.Fprintf(d.current, "BT /%s %.1f Tf 1 0 0 1 %.2f %.2f Tm (%s) Tj ET\n",
			f, size, marginLeft+indent, pageHeight-d.y-size, escape(line))
		d.y += size * 1.35
	}
}

// rule draws a hairline across the content width.
func (d *doc) rule() {
	if d.y+6 > pageHeight-marginBottom {
		d.newPage()
	}
	fmt.Fprintf(d.current, "0.75 w 0.6 0.6 0.6 RG %.2f %.2f m %.2f %.2f l S\n",
		marginLeft, pageHeight-d.y, pageWidth-marginRight, pageHeight-d.y)
	d.y += 8
}

// render assembles the PDF file.
func (d *doc) render() []byte {
	// Stamp the footer on every page before writing them out, so a page that
	// was created by an overflow gets one too.
	for i, p := range d.pages {
		footer := fmt.Sprintf("%s  ·  %d / %d", d.footer, i+1, len(d.pages))
		fmt.Fprintf(p, "BT /%s 7.5 Tf 0.35 0.35 0.35 rg 1 0 0 1 %.2f %.2f Tm (%s) Tj ET\n",
			Italic, marginLeft, marginBottom-28, escape(footer))
	}

	var out bytes.Buffer
	offsets := []int{}
	obj := func(body string) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}

	out.WriteString("%PDF-1.4\n")

	// 1: catalog, 2: pages, 3..: fonts, then page + content pairs.
	pageObjStart := 6
	kids := make([]string, len(d.pages))
	for i := range d.pages {
		kids[i] = fmt.Sprintf("%d 0 R", pageObjStart+i*2)
	}

	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(d.pages)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Oblique /Encoding /WinAnsiEncoding >>")

	for i, p := range d.pages {
		contentRef := pageObjStart + i*2 + 1
		obj(fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] "+
				"/Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>",
			pageWidth, pageHeight, contentRef))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", p.Len(), p.String()))
	}

	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets)+1, xref)
	return out.Bytes()
}

// escape makes a Go string safe inside a PDF literal string, and encodes it as
// WinAnsi (Latin-1 plus the printable range PDF's standard encoding adds).
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		case '\n', '\r', '\t':
			b.WriteByte(' ')
		default:
			switch {
			case r < 32:
				// Control characters have no glyph; a space keeps the line
				// metrics honest.
				b.WriteByte(' ')
			case r < 127:
				b.WriteRune(r)
			case r <= 255:
				// Latin-1 range: WinAnsi agrees with Latin-1 here.
				fmt.Fprintf(&b, "\\%03o", r)
			default:
				if c, ok := winAnsiHigh[r]; ok {
					fmt.Fprintf(&b, "\\%03o", c)
					continue
				}
				// A character this encoding cannot represent is replaced
				// visibly rather than dropped: a silently missing character in
				// a legal document is worse than an obvious placeholder.
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

// winAnsiHigh maps the few characters WinAnsi places outside Latin-1 — the
// typographic quotes and dashes that Italian and French copy actually use.
var winAnsiHigh = map[rune]byte{
	'€': 128, '‚': 130, 'ƒ': 131, '„': 132, '…': 133,
	'†': 134, '‡': 135, 'ˆ': 136, '‰': 137, 'Š': 138,
	'‹': 139, 'Œ': 140, 'Ž': 142, '‘': 145, '’': 146,
	'“': 147, '”': 148, '•': 149, '–': 150, '—': 151,
	'˜': 152, '™': 153, 'š': 154, '›': 155, 'œ': 156,
	'ž': 158, 'Ÿ': 159,
}

// wrap breaks s into lines that fit within width at the given size, measuring
// with the real Helvetica advance widths.
func wrap(s string, f Font, size, width float64) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var (
		lines []string
		line  string
	)
	for _, w := range words {
		candidate := w
		if line != "" {
			candidate = line + " " + w
		}
		if textWidth(candidate, f, size) <= width || line == "" {
			line = candidate
			continue
		}
		lines = append(lines, line)
		line = w
	}
	return append(lines, line)
}

// textWidth is the advance width of s in points.
func textWidth(s string, f Font, size float64) float64 {
	widths := helvetica
	if f == Bold {
		widths = helveticaBold
	}
	var total float64
	for _, r := range s {
		idx := 0
		if r >= 32 && r <= 255 {
			idx = int(r)
		} else {
			idx = 'x'
		}
		total += float64(widths[idx])
	}
	return total * size / 1000
}

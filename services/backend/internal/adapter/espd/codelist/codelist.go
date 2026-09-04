// Package codelist holds the ESPD-EDM criterion definitions this service
// serializes with, vendored per version from the official releases.
//
// It exists because the one thing that genuinely differs between ESPD-EDM 2.1.1
// and 4.1.0 is the CODE LISTS: the criterion UUIDs are stable, but the taxonomy
// codes were shortened ("CRITERION.EXCLUSION.CONVICTIONS.FRAUD" became "fraud")
// and every property UUID a response validates against was renumbered. One
// composed espd.Response, two tables, two serializers — that asymmetry is the
// whole argument for the split.
//
// Neither the table nor the XML fragments are written by hand: both are
// generated from the official sample responses by ./generate, which reads a
// checkout of https://github.com/OP-TED/espd-edm. Re-run it against a newer
// release rather than editing the generated files.
//
// Attribution: the criterion definitions redistributed here are © European
// Union, licensed under the EUPL-1.2. See NOTICE.
package codelist

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// Criterion is one criterion's identity in one ESPD-EDM version.
type Criterion struct {
	// UUID is cbc:ID on the criterion element — stable across versions.
	UUID string
	// TypeCode is cbc:CriterionTypeCode, which is NOT stable across versions.
	TypeCode string
	Name     string
	// AnswerPropertyID is the property UUID this criterion's answer validates,
	// and AnswerDataType the ESPD data type it expects — INDICATOR for the
	// "Your answer?" question every exclusion ground hangs off, LOT_IDENTIFIER
	// for the lots criterion, which asks for an id rather than a yes/no.
	AnswerPropertyID string
	AnswerDataType   string
	// SelfCleaningIndicatorID / SelfCleaningTextID are the Art. 57(6) pair,
	// present on exclusion grounds only. An empty pair means this criterion has
	// no self-cleaning question, and a self-cleaning text for it is dropped
	// rather than attached to the wrong property.
	SelfCleaningIndicatorID string
	SelfCleaningTextID      string
}

//go:embed edm211.xml
var edm211Definitions string

//go:embed edm4.xml
var edm4Definitions string

// Table is one version's complete vendored code list.
type Table struct {
	Version espd.Version
	// Definitions are the verbatim <cac:TenderingCriterion> elements, in the
	// order the generator emitted them. A response carries the definitions it
	// answers, which is what the official samples do and what the reading tools
	// expect: the property tree is read out of the document itself.
	definitions map[espd.CriterionKey]string
	criteria    map[espd.CriterionKey]Criterion
}

// EDM211 and EDM4 are the two vendored tables.
func EDM211() *Table { return newTable(espd.EDM211, edm211Criteria, edm211Definitions) }
func EDM4() *Table   { return newTable(espd.EDM4, edm4Criteria, edm4Definitions) }

func newTable(v espd.Version, criteria map[espd.CriterionKey]Criterion, definitions string) *Table {
	return &Table{Version: v, criteria: criteria, definitions: splitDefinitions(definitions)}
}

// splitDefinitions indexes the embedded fragment by the criterion key the
// generator wrote in the comment above each element.
func splitDefinitions(doc string) map[espd.CriterionKey]string {
	out := map[espd.CriterionKey]string{}
	const open = "<!-- "
	for rest := doc; ; {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, " -->")
		if j < 0 {
			return out
		}
		key := espd.CriterionKey(strings.TrimSpace(rest[:j]))
		rest = rest[j+len(" -->"):]
		end := strings.Index(rest, "</cac:TenderingCriterion>")
		if end < 0 {
			return out
		}
		end += len("</cac:TenderingCriterion>")
		out[key] = strings.TrimSpace(rest[:end])
		rest = rest[end:]
	}
}

// Lookup returns the criterion's identity in this version, or an error naming
// both the criterion and the version. A missing entry is never a silently
// omitted element: a DGUE that looks complete and is missing a criterion the
// buyer asked about is worse than no DGUE at all.
func (t *Table) Lookup(k espd.CriterionKey) (Criterion, error) {
	c, ok := t.criteria[k]
	if !ok {
		return Criterion{}, &espd.CriterionUnsupportedError{Key: k, Version: t.Version}
	}
	return c, nil
}

// Definition returns the verbatim criterion element for k.
func (t *Table) Definition(k espd.CriterionKey) (string, error) {
	d, ok := t.definitions[k]
	if !ok {
		return "", &espd.CriterionUnsupportedError{Key: k, Version: t.Version}
	}
	return d, nil
}

// Keys reports how many criteria the table carries — used by the tests that
// pin the two tables against each other.
func (t *Table) Keys() []espd.CriterionKey {
	out := make([]espd.CriterionKey, 0, len(t.criteria))
	for k := range t.criteria {
		out = append(out, k)
	}
	return out
}

// Validate reports a table whose Go entries and embedded XML have drifted
// apart — which can only happen if someone edited a generated file by hand.
func (t *Table) Validate() error {
	for k := range t.criteria {
		if _, ok := t.definitions[k]; !ok {
			return fmt.Errorf("codelist %s: %q has an entry but no definition", t.Version, k)
		}
	}
	for k := range t.definitions {
		if _, ok := t.criteria[k]; !ok {
			return fmt.Errorf("codelist %s: %q has a definition but no entry", t.Version, k)
		}
	}
	return nil
}

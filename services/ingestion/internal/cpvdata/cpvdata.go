// Package cpvdata carries the European Commission's CPV 2008 vocabulary — every
// code's label in all 24 official EU languages — as a committed, embedded
// artifact.
//
// It is committed rather than fetched because it is the bridge cross-language
// search depends on: a CPV code means the same thing in every EU language, so
// mapping a query onto codes finds notices written in languages the query is
// not. An install that has to reach the network for that would make retrieval
// quality depend on a third-party site being up.
//
// It is safe to commit because it is static: CPV 2008 has not been revised in
// years, and a revision is a deliberate re-run of scripts/cpv/convert.mjs plus a
// re-seed, not a background update.
package cpvdata

import (
	"bytes"
	"compress/gzip"
	_ "embed" // required: //go:embed on a []byte needs the package imported, even unused otherwise
	"encoding/csv"
	"fmt"
	"io"
)

//go:embed cpv-2008.csv.gz
var CSV []byte

// Row is one code's label in one language.
type Row struct {
	Code  string
	Lang  string // lowercase ISO 639-1
	Label string
}

// Rows decodes the embedded vocabulary.
//
// It reads the whole thing into memory (~227k rows, a few tens of MB of Go
// strings) because the only caller is a one-off seeding command that then bulk
// inserts. Streaming would complicate the caller for no benefit at that size.
func Rows() ([]Row, error) {
	zr, err := gzip.NewReader(bytes.NewReader(CSV))
	if err != nil {
		return nil, fmt.Errorf("cpvdata: gunzip: %w", err)
	}
	defer zr.Close()

	r := csv.NewReader(zr)
	r.FieldsPerRecord = 3
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("cpvdata: read header: %w", err)
	}
	if len(header) != 3 || header[0] != "code" || header[1] != "lang" || header[2] != "label" {
		return nil, fmt.Errorf("cpvdata: header = %v, want [code lang label]", header)
	}

	var out []Row
	for line := 2; ; line++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cpvdata: read line %d: %w", line, err)
		}
		out = append(out, Row{Code: rec[0], Lang: rec[1], Label: rec[2]})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cpvdata: vocabulary is empty")
	}
	return out, nil
}

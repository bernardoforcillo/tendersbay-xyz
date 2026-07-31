package eval

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/fs"
	"time"
)

// CorpusTender is one tender in the evaluation snapshot.
//
// The snapshot exists because relevance cannot be measured against the
// production database: it changes every day, so the same query would score
// differently tomorrow for reasons that have nothing to do with the code. The
// fields mirror the columns the search paths actually read, so a snapshot can
// be loaded into an empty database and produce the same rankings.
type CorpusTender struct {
	Source        string     `json:"source"`
	SourceRef     string     `json:"source_ref"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	BuyerName     string     `json:"buyer_name,omitempty"`
	Status        string     `json:"status,omitempty"`
	ProcedureType string     `json:"procedure_type,omitempty"`
	Language      string     `json:"language,omitempty"`
	Country       string     `json:"country,omitempty"`
	NUTS          string     `json:"nuts,omitempty"`
	CPV           string     `json:"cpv,omitempty"`
	CPVSecondary  []string   `json:"cpv_secondary,omitempty"`
	Value         *int64     `json:"value,omitempty"`
	Currency      string     `json:"currency,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	Deadline      *time.Time `json:"deadline,omitempty"`
}

// Key is the stable identity judgements are written against.
func (c CorpusTender) Key() string { return c.Source + ":" + c.SourceRef }

// maxCorpusLine bounds one JSONL record. A description is truncated on export,
// so a line far larger than this means the file is not what we think it is.
const maxCorpusLine = 1 << 20

// LoadCorpus reads a gzipped JSONL snapshot.
//
// JSONL rather than one JSON array so the exporter can stream, and gzipped
// because the uncompressed form is several times larger for no benefit and
// this file is committed. (Not because descriptions dominate the size, as an
// earlier version of this comment claimed: 0 of the 3,030 tenders in the
// committed snapshot carry a description at all — see eval/README.md's
// "Corpus limitation" section. The size here is titles/buyer names/CPV codes
// repeated ~3,000 times, which gzip still compresses well.)
func LoadCorpus(fsys fs.FS, name string) ([]CorpusTender, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("eval: open corpus %s: %w", name, err)
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("eval: gunzip corpus %s: %w", name, err)
	}
	defer zr.Close()

	var (
		out  []CorpusTender
		seen = map[string]bool{}
	)
	scanner := bufio.NewScanner(zr)
	scanner.Buffer(make([]byte, 0, 64*1024), maxCorpusLine)
	for line := 1; scanner.Scan(); line++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var t CorpusTender
		if err := json.Unmarshal(scanner.Bytes(), &t); err != nil {
			return nil, fmt.Errorf("eval: parse corpus %s line %d: %w", name, line, err)
		}
		if t.Source == "" || t.SourceRef == "" {
			return nil, fmt.Errorf("eval: corpus %s line %d has no source/source_ref; judgements key on it", name, line)
		}
		if seen[t.Key()] {
			return nil, fmt.Errorf("eval: corpus %s line %d duplicates key %s", name, line, t.Key())
		}
		seen[t.Key()] = true
		out = append(out, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("eval: read corpus %s: %w", name, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("eval: corpus %s is empty", name)
	}
	return out, nil
}

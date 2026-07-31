package eval

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// evalK is the ranking window every metric is reported at. 20 because that is
// the frontend's PAGE_SIZE — measuring deeper would score results no user sees.
const evalK = 20

// regressionTolerance is how far a metric may drift before the harness fails.
//
// It is not zero because embedding output is not bit-stable across runs (model
// server version, batching, floating-point accumulation), so a hair's-width
// move is noise. A harness that fails on noise is a harness everyone learns to
// ignore, which is worse than no harness.
const regressionTolerance = 0.01

// allowAllLimiter satisfies tender.RateLimiter. The harness issues ~50 searches
// back to back; a real limiter would reject most of them and the run would
// measure the limiter instead of the ranking.
type allowAllLimiter struct{}

func (allowAllLimiter) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}

// kbAdapter bridges go-services/knowledge to tender.KnowledgeBase, whose
// methods are declared in tender's own ScoredChunk/Filters types rather than
// knowledge's. main.go's unexported knowledgeBaseAdapter is the production
// equivalent this mirrors; duplicating it here beats widening the composition
// root's API for a fixture.
//
// It differs from the task brief's guess in two ways, both required for this
// to compile against the real package:
//   - knowledge.KnowledgeBase has no RelatedByDocID/SearchByVector that return
//     []tender.ScoredChunk directly — both return []knowledge.SearchResult
//     (a rag.Chunk embedding DocID, plus Score), so the conversion loop below
//     is still needed, just over that shape instead of a bespoke "hits" type.
//   - there is no package-level knowledgeFilters function; main.go's
//     equivalent is named vectorFilter and does not carry Buyer across (buyer
//     name is not indexed in Qdrant's payload) — knowledgeFilters below
//     mirrors that mapping exactly.
type kbAdapter struct{ kb *knowledge.KnowledgeBase }

func (a kbAdapter) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return a.kb.EmbedQuery(ctx, query)
}

func (a kbAdapter) SearchByVector(ctx context.Context, vec []float32, limit int, f tender.Filters) ([]tender.ScoredChunk, error) {
	hits, err := a.kb.SearchByVector(ctx, vec, limit, knowledgeFilters(f))
	if err != nil {
		return nil, err
	}
	out := make([]tender.ScoredChunk, len(hits))
	for i, h := range hits {
		out[i] = tender.ScoredChunk{DocID: h.DocID, Score: h.Score}
	}
	return out, nil
}

func (a kbAdapter) RelatedByDocID(ctx context.Context, docID string, limit int) ([]tender.ScoredChunk, error) {
	hits, err := a.kb.RelatedByDocID(ctx, docID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tender.ScoredChunk, len(hits))
	for i, h := range hits {
		out[i] = tender.ScoredChunk{DocID: h.DocID, Score: h.Score}
	}
	return out, nil
}

// knowledgeFilters maps the domain's filters onto the vector store's payload
// filter, mirroring main.go's vectorFilter. Buyer is deliberately not carried
// across: buyer_name is not a point-payload field (see knowledge.Attributes),
// and this filter is only ever an optimisation — Postgres applies the full
// filter regardless, so dropping a constraint here costs some wasted
// candidates rather than hiding a match.
func knowledgeFilters(f tender.Filters) knowledge.SearchFilter {
	return knowledge.SearchFilter{
		Countries:    f.Countries,
		Statuses:     f.Statuses,
		CPVPrefixes:  f.CPVPrefixes,
		NUTSPrefixes: f.NUTSPrefixes,
		ValueMin:     f.ValueMin,
		ValueMax:     f.ValueMax,
		DeadlineFrom: f.DeadlineFrom,
		DeadlineTo:   f.DeadlineTo,
	}
}

// harness holds everything one run needs.
type harness struct {
	svc    *tender.Service
	corpus map[string]CorpusTender
	golden GoldenSet
}

// newHarness wires a real Service against the evaluation database and Qdrant
// collection, or skips.
//
// The guard is three separate env vars rather than one flag because all three
// services must be reachable for the run to mean anything, and a partial setup
// should say which piece is missing rather than fail deep inside a search.
func newHarness(t *testing.T) harness {
	t.Helper()

	dsn := requireEnv(t, "EVAL_DATABASE_URL")
	qdrantURL := requireEnv(t, "EVAL_QDRANT_URL")
	ollamaURL := requireEnv(t, "EVAL_OLLAMA_URL")
	model := os.Getenv("EVAL_EMBEDDING_MODEL")
	if model == "" {
		model = "embeddinggemma:latest"
	}

	ctx := context.Background()
	db, sqlDB, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	kb, err := knowledge.NewKnowledgeBase(ctx, qdrantURL, ollamaURL, model)
	if err != nil {
		t.Fatalf("knowledge.NewKnowledgeBase: %v", err)
	}

	fsys := os.DirFS("testdata")
	golden, err := LoadGolden(fsys, "golden.json")
	if err != nil {
		t.Fatalf("LoadGolden: %v", err)
	}
	corpusRows, err := LoadCorpus(fsys, "corpus.jsonl.gz")
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	corpus := make(map[string]CorpusTender, len(corpusRows))
	for _, c := range corpusRows {
		corpus[c.Key()] = c
	}

	// Production tiers and ranking, so the harness measures what users get.
	// A bespoke config here would produce numbers that describe nothing shipped.
	svc := tender.NewService(
		postgres.NewTenderRepo(db),
		kbAdapter{kb: kb},
		allowAllLimiter{},
		nil, // no ProfileSource: the harness never annotates for a client
		tender.Config{
			AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 1 << 30, RateWindow: time.Minute},
			AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 1 << 30, RateWindow: time.Minute},
			Ranking:    tender.DefaultRanking(),
		},
		// Same attachment main.go makes: unconditional, since an unseeded
		// cpv_terms table simply resolves no codes and the arm contributes
		// nothing (see tender.Service.cpvCandidates). Without this the harness
		// measures a Service with the CPV arm permanently off, regardless of
		// what main.go wires — a bespoke omission here would be exactly the
		// "numbers that describe nothing shipped" the comment above warns against.
	).WithCPVLexicon(postgres.NewCPVLexicon(db))

	return harness{svc: svc, corpus: corpus, golden: golden}
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s not set", name)
	}
	return v
}

// run executes every golden query and scores it.
func (h harness) run(t *testing.T) Report {
	t.Helper()
	ctx := context.Background()

	results := make([]QueryResult, 0, len(h.golden))
	// modeCounts is diagnostic only — never written to Report/baseline.json,
	// which have no such field. It exists because a search silently falling
	// back to ModeLexical (dense arm down) still returns a ranked list and a
	// non-zero recall, so nothing in the recorded metrics would flag it; only
	// the mode itself proves the run actually measured hybrid retrieval.
	modeCounts := map[tender.RetrievalMode]int{}
	for _, q := range h.golden {
		out, err := h.svc.Search(ctx, tender.SearchParams{
			Query: q.Query,
			// True because these are strings a person typed into a search box —
			// the same choice connectapi's handler makes, for the same reason.
			ParseQuery:    true,
			Limit:         evalK,
			Authenticated: true,
			RateLimitKey:  "eval:" + q.ID,
		})
		if err != nil {
			t.Fatalf("Search(%q): %v", q.ID, err)
		}
		modeCounts[out.Mode]++

		ranked := make([]string, len(out.Results))
		for i, r := range out.Results {
			ranked[i] = r.Source + ":" + r.SourceRef
		}

		results = append(results, QueryResult{
			ID:            q.ID,
			QueryLanguage: q.Language,
			Scores: Scores{
				Recall:  RecallAtK(ranked, q.Judgements, evalK),
				NDCG:    NDCGAtK(ranked, q.Judgements, evalK),
				MRR:     MRR(ranked, q.Judgements),
				Queries: 1,
			},
			ByDocLanguage: h.byDocLanguage(ranked, q),
		})
	}
	for mode, n := range modeCounts {
		t.Logf("retrieval_mode %-10s %d/%d queries", mode, n, len(h.golden))
	}
	return BuildReport(evalK, results)
}

// byDocLanguage scores the query once per language its relevant tenders are
// written in.
//
// The ranked list is NOT filtered — only the judgements are. That is the
// measurement that matters: "of the German notices this Italian query should
// have found, how many did it actually surface, competing against everything
// else?" Filtering the ranked list too would score a German-only sub-ranking
// that no user ever sees.
func (h harness) byDocLanguage(ranked []string, q GoldenQuery) map[string]Scores {
	perLang := map[string]Judgements{}
	for key, grade := range q.Judgements {
		if grade <= 0 {
			continue
		}
		lang := h.corpus[key].Language
		if lang == "" {
			// A tender with no recorded language cannot be attributed to a cell.
			// Counting it under "" would create a phantom column; it still
			// contributes to the query's overall score.
			continue
		}
		if perLang[lang] == nil {
			perLang[lang] = Judgements{}
		}
		perLang[lang][key] = grade
	}

	out := make(map[string]Scores, len(perLang))
	for lang, j := range perLang {
		out[lang] = Scores{
			Recall:  RecallAtK(ranked, j, evalK),
			NDCG:    NDCGAtK(ranked, j, evalK),
			MRR:     MRR(ranked, j),
			Queries: 1,
		}
	}
	return out
}

// TestHarness_NoRegressionAgainstBaseline is the gate every ranking change must
// pass. Set EVAL_WRITE_BASELINE=1 to record the current scores instead of
// comparing — do that only in the same commit as the change that moves them, so
// every relevance change is reviewable as a numeric diff.
func TestHarness_NoRegressionAgainstBaseline(t *testing.T) {
	h := newHarness(t)
	current := h.run(t)

	// Always print, pass or fail: an intentional trade (recall up, precision
	// down) has to be a visible decision rather than a silent one.
	t.Log("\n" + current.Format())

	if os.Getenv("EVAL_WRITE_BASELINE") == "1" {
		raw, err := MarshalBaseline(current)
		if err != nil {
			t.Fatalf("MarshalBaseline: %v", err)
		}
		const path = "testdata/baseline.json"
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote %s — commit it alongside the change that moved it", path)
		return
	}

	baseline, err := LoadBaseline(os.DirFS("testdata"), "baseline.json")
	if err != nil {
		t.Fatalf("LoadBaseline: %v — run once with EVAL_WRITE_BASELINE=1 to record it", err)
	}
	if baseline.K != current.K {
		t.Fatalf("baseline k=%d, current k=%d — scores at different windows are not comparable", baseline.K, current.K)
	}

	regressions := Compare(baseline, current, regressionTolerance)
	for _, r := range regressions {
		t.Errorf("regression: %-16s %-12s baseline=%.4f current=%.4f", r.Scope, r.Metric, r.Baseline, r.Current)
	}
	if len(regressions) > 0 {
		t.Errorf("%d regressions beyond the %.3f tolerance", len(regressions), regressionTolerance)
	}
}

// TestHarness_ReportsCrossLanguageCells fails when the matrix has no
// off-diagonal cell at all.
//
// Separate from the regression gate on purpose: a baseline recorded from a run
// that measured nothing cross-language would then "pass" forever while the
// requirement went unmeasured.
func TestHarness_ReportsCrossLanguageCells(t *testing.T) {
	h := newHarness(t)
	report := h.run(t)

	cross := 0
	for qLang, row := range report.Matrix {
		for dLang := range row {
			if qLang != dLang {
				cross++
			}
		}
	}
	if cross == 0 {
		t.Fatalf("the report has no cross-language cells:\n%s", report.Format())
	}
	t.Logf("%d cross-language cells measured", cross)
}

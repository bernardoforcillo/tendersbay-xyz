// Internal tests for the parts of this package that are deliberately
// unexported: file ranking, address normalisation, and the observation payload.
// All three are policy that nothing outside this package may reach, and each of
// them is the thing under test rather than a helper of it.
package retrieve

import (
	"reflect"
	"strings"
	"testing"
)

// TestRankFilesPutsTheGoverningDocumentFirst is the ranking's whole job: when a
// listing publishes more files than the cap, the one carrying the scoring grid
// has to be inside it. A pass that reads eight "Modello A-H" attachments and
// misses the disciplinare reports success and delivers nothing.
func TestRankFilesPutsTheGoverningDocumentFirst(t *testing.T) {
	files := []FileLink{
		{URL: "https://p.example/a.pdf", Label: "Modello A — Domanda di partecipazione"},
		{URL: "https://p.example/b.pdf", Label: "Patto di integrità"},
		{URL: "https://p.example/c.pdf", Label: "Bando di gara"},
		{URL: "https://p.example/d.pdf", Label: "Disciplinare di gara"},
		{URL: "https://p.example/e.pdf", Label: "Criteri di valutazione dell'offerta"},
	}

	got := labelsOf(rankFiles(files, 3))
	want := []string{"Disciplinare di gara", "Criteri di valutazione dell'offerta", "Bando di gara"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rankFiles = %v, want %v", got, want)
	}
}

// TestRankFilesReadsTheFilenameWhenTheLabelSaysNothing covers the portals whose
// every download column reads "Scarica": the label carries no signal at all, and
// the only thing distinguishing the disciplinare is the name of the file.
func TestRankFilesReadsTheFilenameWhenTheLabelSaysNothing(t *testing.T) {
	files := []FileLink{
		{URL: "https://p.example/allegato_1.pdf", Label: "Scarica"},
		{URL: "https://p.example/Capitolato_Speciale_Appalto.pdf", Label: "Scarica"},
	}

	got := rankFiles(files, 1)
	if len(got) != 1 || !strings.Contains(got[0].URL, "Capitolato") {
		t.Errorf("rankFiles = %+v, want the capitolato first", got)
	}
}

// TestRankFilesIsNotAFilter is the property that keeps a listing of "Allegato
// 1..8" from degrading to nothing. Ranking decides ORDER when the cap binds and
// decides nothing else; an unmatched file is still fetched if there is room.
func TestRankFilesIsNotAFilter(t *testing.T) {
	files := []FileLink{
		{URL: "https://p.example/1.pdf", Label: "Allegato 1"},
		{URL: "https://p.example/2.pdf", Label: "Allegato 2"},
		{URL: "https://p.example/3.pdf", Label: "Allegato 3"},
	}

	got := labelsOf(rankFiles(files, MaxFilesPerTender))
	want := []string{"Allegato 1", "Allegato 2", "Allegato 3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rankFiles = %v, want published order %v", got, want)
	}
}

// TestRankFilesIsStableWithinAClass keeps the page's own ordering, which is real
// information: a buyer lists the governing document first far more often than
// not, so a keyword list should refine that order rather than replace it.
func TestRankFilesIsStableWithinAClass(t *testing.T) {
	files := []FileLink{
		{URL: "https://p.example/1.pdf", Label: "Disciplinare di gara"},
		{URL: "https://p.example/2.pdf", Label: "Disciplinare — allegato tecnico"},
		{URL: "https://p.example/3.pdf", Label: "Capitolato speciale"},
	}

	got := labelsOf(rankFiles(files, 3))
	want := labelsOf(files)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rankFiles = %v, want the published order preserved %v", got, want)
	}
}

// TestClassifyMatchesWholeWordsOnly is why "csa" is matchable at all. As a plain
// substring, a three-letter abbreviation fires on any word containing those
// letters and would promote an unrelated attachment over the real disciplinare.
func TestClassifyMatchesWholeWordsOnly(t *testing.T) {
	for _, tc := range []struct {
		label string
		want  fileClass
	}{
		{"CSA - Capitolato Speciale d'Appalto", classGoverning},
		{"CSA.pdf", classGoverning},
		{"Relazione paesaggistica", classOther},
		{"Elenco prezzi", classOther},
	} {
		if got := classify(FileLink{URL: "https://p.example/x", Label: tc.label}); got != tc.want {
			t.Errorf("classify(%q) = %d, want %d", tc.label, got, tc.want)
		}
	}
}

// TestDedupeFilesCollapsesSessionVariantsOfOneDocument stops the same file being
// downloaded twice — once from the table row and once from the sidebar icon,
// each carrying a different session id — which would spend two of the fifteen
// slots on one document.
func TestDedupeFilesCollapsesSessionVariantsOfOneDocument(t *testing.T) {
	files := []FileLink{
		{URL: "https://p.example/dl?id=7&sid=aaa", Label: "Disciplinare"},
		{URL: "https://p.example/dl?id=7&sid=bbb", Label: "Scarica"},
		{URL: "https://p.example/dl?id=8&sid=ccc", Label: "Capitolato"},
	}

	got := dedupeFiles(files)
	if len(got) != 2 {
		t.Fatalf("dedupeFiles returned %d links, want 2: %+v", len(got), got)
	}
	// The FIRST occurrence survives, verbatim: it is the one whose label carries
	// the words ranking reads, and the address still has to be requested as the
	// page published it.
	if got[0].URL != "https://p.example/dl?id=7&sid=aaa" || got[0].Label != "Disciplinare" {
		t.Errorf("kept %+v, want the first published occurrence verbatim", got[0])
	}
}

// TestNormaliseURLDropsSessionParametersAndSortsTheRest pins what makes a stored
// document URL stable across runs. Without it, FinishRetrieval's
// delete-what-is-missing rule churns every row on every retrieval.
func TestNormaliseURLDropsSessionParametersAndSortsTheRest(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"https://p.example/dl?sid=abc&id=7", "https://p.example/dl?id=7"},
		{"https://p.example/dl;jsessionid=x?b=2&a=1", "https://p.example/dl;jsessionid=x?a=1&b=2"},
		{"https://p.example/dl?JSESSIONID=x&id=7", "https://p.example/dl?id=7"},
		{"https://p.example/gare/1#top", "https://p.example/gare/1"},
		{"https://p.example/gare/1", "https://p.example/gare/1"},
	} {
		if got := normaliseURL(tc.in); got != tc.want {
			t.Errorf("normaliseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestIsTEDEntryRecognisesTheRegisterItself keeps this pass from re-ingesting a
// notice PDF that is already in the corpus under a different type.
func TestIsTEDEntryRecognisesTheRegisterItself(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://ted.europa.eu/en/notice/-/detail/545620-2026", true},
		{"https://api.ted.europa.eu/v3/notices/545620-2026", true},
		{"https://www.comune.example.it/bandi/1", false},
		{"https://notted.europa.eu.example.com/x", false},
	} {
		if got := isTEDEntry(tc.url); got != tc.want {
			t.Errorf("isTEDEntry(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

// TestObservationCarriesNoIdentifyingProperty is the privacy rule as an
// assertion rather than a convention.
//
// The payload may hold counts, booleans and low-cardinality categoricals and
// nothing else — no URL, no host, no buyer name. Adding one would require
// widening the struct, which is a change a reviewer can see; this test is what
// makes it a change a reviewer is FORCED to see.
func TestObservationCarriesNoIdentifyingProperty(t *testing.T) {
	o := observation{
		country: "IT", platform: "tuttogare", status: StatusOK,
		filesFound: 9, filesExtracted: 4, filesCapped: false,
		partsWritten: 61, bytesFetched: 3 << 20,
	}

	attrs := o.attrs()
	if len(attrs)%2 != 0 {
		t.Fatalf("attrs has %d entries, which is not key/value pairs", len(attrs))
	}

	got := map[string]any{}
	for i := 0; i < len(attrs); i += 2 {
		got[attrs[i].(string)] = attrs[i+1]
	}

	want := []string{
		"location", "country", "platform", "status",
		"files_found", "files_extracted", "files_capped", "parts_written", "bytes_bucket",
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("missing property %q", key)
		}
	}
	if len(got) != len(want) {
		t.Errorf("attrs has %d properties, want exactly %d: %v", len(got), len(want), got)
	}

	// has_documents is deliberately absent: it would report what we SAW rather
	// than what we can READ, and the gap between those two numbers is the whole
	// finding of this pass.
	if _, ok := got["has_documents"]; ok {
		t.Error("has_documents is present; files_found and files_extracted exist precisely so it is not")
	}
	for _, forbidden := range []string{"url", "documents_url", "host", "buyer", "buyer_name", "filename"} {
		if _, ok := got[forbidden]; ok {
			t.Errorf("observation carries %q, which is identifying or free text", forbidden)
		}
	}
	if got["bytes_bucket"] != "1-10m" {
		t.Errorf("bytes_bucket = %v, want 1-10m", got["bytes_bucket"])
	}
}

// TestObservationReportsUnmeasuredCountsAsAbsent keeps a refusal from reading as
// an empty portal in the event stream, the same way Counts keeps it out of the
// columns. A robots-denied tender did not find zero files; we never saw its
// listing.
func TestObservationReportsUnmeasuredCountsAsAbsent(t *testing.T) {
	got := newObservation(PendingTender{Country: "IT"}, Outcome{Status: StatusRobotsDenied})

	if got.filesFound != noCount || got.filesExtracted != noCount {
		t.Errorf("counts = %d/%d, want %d/%d", got.filesFound, got.filesExtracted, noCount, noCount)
	}
	if got.filesCapped {
		t.Error("filesCapped is true for a tender whose listing was never read")
	}
}

// TestObservationFlagsACappedListing makes a truncation visible in the event
// stream, so a capped tender is not silently indistinguishable from a fully-read
// one in every aggregate.
func TestObservationFlagsACappedListing(t *testing.T) {
	got := newObservation(PendingTender{Country: "IT"}, Outcome{
		Status: StatusOK, Platform: "tuttogare",
		FilesFound: MaxFilesPerTender + 25, FilesExtracted: MaxFilesPerTender,
	})

	if !got.filesCapped {
		t.Error("filesCapped is false for a listing larger than the per-tender cap")
	}
	if got.filesFound != MaxFilesPerTender+25 {
		t.Errorf("filesFound = %d, want the pre-cap listing size", got.filesFound)
	}
}

// labelsOf reduces a ranked slice to the labels the assertions read.
func labelsOf(files []FileLink) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Label
	}
	return out
}

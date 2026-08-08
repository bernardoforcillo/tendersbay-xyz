package retrieve

// The per-tender request budget.
//
// The fetching adapter's per-host pacer bounds how FAST we talk to one server;
// this bounds how MUCH we ask of it for one tender. They are different promises
// and neither implies the other: a framework agreement whose listing publishes
// three hundred attachments would be perfectly paced and still be three hundred
// requests aimed at one comune for one row.
//
// It lives here rather than in the adapter because it is spent here. The
// adapter owns politeness that is a property of the connection — pacing, robots,
// one request at a time per host — and it cannot see a tender at all; the caps
// below are per-tender, so the only place that can enforce them is the loop that
// walks a tender's files. The adapter re-exports the two constants so a reader
// of the fetcher finds them where the rest of its bounds are documented.

const (
	// MaxFilesPerTender caps downloads for one tender. Fifteen covers the
	// observed Italian shape of a procedure — bando, disciplinare, capitolato,
	// DGUE, schema di contratto, patto di integrità, planimetrie, and a handful
	// of modelli — with room to spare. A listing with three hundred files is
	// not a reason to make three hundred requests; it is a reason to record
	// that there were three hundred and read the fifteen that matter, which is
	// what rankFiles decides and Outcome.FilesFound records.
	MaxFilesPerTender = 15

	// MaxTenderBytes caps total downloaded bytes for one tender. It is
	// deliberately not MaxFilesPerTender × the adapter's per-response cap
	// (300 MiB): that product is an arithmetic possibility, not a budget anyone
	// would choose to spend on one row, and the pod this runs in is sized for a
	// working set an order of magnitude smaller.
	MaxTenderBytes = 120 << 20
)

// Budget tracks one tender's consumption against those two caps. It is not safe
// for concurrent use, which matches how it is used: one tender's files are
// fetched serially, because they are all behind one host's single request slot
// anyway.
//
// Allow is consulted before each fetch and Spend recorded after each one. Spend
// takes the byte count rather than Allow reserving it, because a response's
// size is not knowable before it arrives — so the last file of a tender may
// carry the total slightly past MaxTenderBytes, by at most one response. That
// overshoot is bounded by the adapter's per-response cap and is the honest
// trade for not trusting a Content-Length header we did not write.
type Budget struct {
	files int
	bytes int
}

// Allow reports whether another file may be fetched for this tender.
func (b *Budget) Allow() bool {
	return b.files < MaxFilesPerTender && b.bytes < MaxTenderBytes
}

// Spend records one fetched file of n bytes, whether or not any text came out
// of it. A file that downloaded and then failed to extract cost the server
// exactly as much as one that worked, and a budget that only counted successes
// would let a tender of unreadable attachments spend the cap several times
// over.
func (b *Budget) Spend(n int) {
	b.files++
	b.SpendBytes(n)
}

// SpendBytes records bytes that were downloaded for this tender but are not one
// of its files — which in practice means the listing page itself.
//
// The distinction is small and load-bearing. The listing costs the buyer's
// server a request and costs this pod its bytes, so it belongs in the volume
// this tender is reported as having cost. It is not a DOCUMENT, though, and
// charging it against MaxFilesPerTender would silently make the real cap
// fourteen — which is the sort of off-by-one that shows up months later as
// "the last attachment is never read".
func (b *Budget) SpendBytes(n int) {
	if n > 0 {
		b.bytes += n
	}
}

// Files and Bytes report what has been spent so far. They exist for the
// observation, which needs to be able to say that a tender hit the cap rather
// than that it published few documents — the same coverage-versus-cause
// distinction this whole package is built around.
func (b *Budget) Files() int { return b.files }
func (b *Budget) Bytes() int { return b.bytes }

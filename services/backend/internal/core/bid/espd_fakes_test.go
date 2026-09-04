package bid

import (
	"context"
	"fmt"
	"time"
)

// espdMem is the ESPD half of fakeRepo, kept beside it so the lifecycle fake
// stays readable. Keyed by bid id like the real tables.
type espdMem struct {
	lots          map[string][]Lot
	subs          map[string][]Subcontractor
	reliances     map[string][]Reliance
	confirmations map[string]DeclarationConfirmation
	seq           int
}

var espdState = map[*fakeRepo]*espdMem{}

func (r *fakeRepo) espd() *espdMem {
	m, ok := espdState[r]
	if !ok {
		m = &espdMem{
			lots: map[string][]Lot{}, subs: map[string][]Subcontractor{},
			reliances: map[string][]Reliance{}, confirmations: map[string]DeclarationConfirmation{},
		}
		espdState[r] = m
	}
	return m
}

func (m *espdMem) nextID(prefix string) string {
	m.seq++
	return fmt.Sprintf("%s-%d", prefix, m.seq)
}

func (r *fakeRepo) PutLot(_ context.Context, bidID string, l Lot) (Lot, error) {
	m := r.espd()
	l.BidID = bidID
	for i, cur := range m.lots[bidID] {
		if cur.LotRef == l.LotRef {
			l.ID = cur.ID
			m.lots[bidID][i] = l
			return l, nil
		}
	}
	l.ID = m.nextID("lot")
	m.lots[bidID] = append(m.lots[bidID], l)
	return l, nil
}

func (r *fakeRepo) DeleteLot(_ context.Context, bidID, id string) error {
	m := r.espd()
	for i, cur := range m.lots[bidID] {
		if cur.ID == id {
			m.lots[bidID] = append(m.lots[bidID][:i], m.lots[bidID][i+1:]...)
			return nil
		}
	}
	return ErrInvalidArgument
}

func (r *fakeRepo) PutSubcontractor(_ context.Context, bidID string, s Subcontractor) (Subcontractor, error) {
	m := r.espd()
	s.BidID = bidID
	for i, cur := range m.subs[bidID] {
		if cur.VAT == s.VAT {
			s.ID = cur.ID
			m.subs[bidID][i] = s
			return s, nil
		}
	}
	s.ID = m.nextID("sub")
	m.subs[bidID] = append(m.subs[bidID], s)
	return s, nil
}

func (r *fakeRepo) DeleteSubcontractor(_ context.Context, bidID, id string) error {
	m := r.espd()
	for i, cur := range m.subs[bidID] {
		if cur.ID == id {
			m.subs[bidID] = append(m.subs[bidID][:i], m.subs[bidID][i+1:]...)
			return nil
		}
	}
	return ErrInvalidArgument
}

func (r *fakeRepo) PutReliance(_ context.Context, bidID string, rel Reliance) (Reliance, error) {
	m := r.espd()
	rel.BidID = bidID
	for i, cur := range m.reliances[bidID] {
		if cur.VAT == rel.VAT && cur.Criterion == rel.Criterion {
			rel.ID = cur.ID
			m.reliances[bidID][i] = rel
			return rel, nil
		}
	}
	rel.ID = m.nextID("rel")
	m.reliances[bidID] = append(m.reliances[bidID], rel)
	return rel, nil
}

func (r *fakeRepo) DeleteReliance(_ context.Context, bidID, id string) error {
	m := r.espd()
	for i, cur := range m.reliances[bidID] {
		if cur.ID == id {
			m.reliances[bidID] = append(m.reliances[bidID][:i], m.reliances[bidID][i+1:]...)
			return nil
		}
	}
	return ErrInvalidArgument
}

func (r *fakeRepo) ListEspdData(_ context.Context, bidID string) (EspdData, error) {
	m := r.espd()
	return EspdData{Lots: m.lots[bidID], Subcontractors: m.subs[bidID], Reliances: m.reliances[bidID]}, nil
}

func (r *fakeRepo) PutDeclarationConfirmation(_ context.Context, c DeclarationConfirmation) (DeclarationConfirmation, error) {
	if c.ConfirmedAt.IsZero() {
		c.ConfirmedAt = time.Now()
	}
	r.espd().confirmations[c.BidID] = c
	return c, nil
}

func (r *fakeRepo) GetDeclarationConfirmation(_ context.Context, bidID string) (DeclarationConfirmation, error) {
	c, ok := r.espd().confirmations[bidID]
	if !ok {
		return DeclarationConfirmation{}, ErrConfirmationNotFound
	}
	return c, nil
}

// viewFakeRepo never exercises the ESPD methods; it satisfies the port so the
// read/aggregation tests keep compiling.
func (r *viewFakeRepo) PutLot(context.Context, string, Lot) (Lot, error) { return Lot{}, nil }
func (r *viewFakeRepo) DeleteLot(context.Context, string, string) error  { return nil }
func (r *viewFakeRepo) PutSubcontractor(context.Context, string, Subcontractor) (Subcontractor, error) {
	return Subcontractor{}, nil
}
func (r *viewFakeRepo) DeleteSubcontractor(context.Context, string, string) error { return nil }
func (r *viewFakeRepo) PutReliance(context.Context, string, Reliance) (Reliance, error) {
	return Reliance{}, nil
}
func (r *viewFakeRepo) DeleteReliance(context.Context, string, string) error { return nil }
func (r *viewFakeRepo) ListEspdData(context.Context, string) (EspdData, error) {
	return EspdData{}, nil
}
func (r *viewFakeRepo) PutDeclarationConfirmation(_ context.Context, c DeclarationConfirmation) (DeclarationConfirmation, error) {
	return c, nil
}
func (r *viewFakeRepo) GetDeclarationConfirmation(context.Context, string) (DeclarationConfirmation, error) {
	return DeclarationConfirmation{}, ErrConfirmationNotFound
}

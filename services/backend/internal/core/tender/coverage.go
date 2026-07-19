package tender

import "context"

// Coverage returns the alpha-2 countries we currently hold tenders for
// (DISTINCT country over ingested_tenders). Anonymous, unmetered — the
// landing marquee's only caller. "Available" is TED-inclusive: any ingested
// tender for a country counts.
func (s *Service) Coverage(ctx context.Context) ([]string, error) {
	return s.repo.DistinctCountries(ctx)
}

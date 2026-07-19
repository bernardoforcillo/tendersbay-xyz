package tender

import "strings"

// EUThreshold holds the 2026-2027 EU procurement thresholds in MINOR UNITS
// (cents). Effective-dated + config-injected so a biennial revision is a
// one-line change here, never in the classifier. Distinct from FitThresholds
// (the fit-agent's relevance knobs) — this is the statutory EU directive band.
type EUThreshold struct {
	WorksMinor              int64 // works (CPV division 45xxxxxx), single threshold
	SuppliesCentralMinor    int64 // supplies/services, central-govt authorities (lower bound)
	SuppliesSubCentralMinor int64 // supplies/services, sub-central authorities (upper bound)
}

// euThreshold coarsely classifies a tender as below/above the EU procurement
// threshold — the buyer-agnostic, SME-relevant band. Because the tender model
// carries neither contract-nature (beyond CPV) nor buyer-type, supplies/
// services tenders whose value falls BETWEEN the central and sub-central
// thresholds are genuinely undecidable and return "" (no badge) rather than a
// guess. A nil value also returns "". Works (CPV division 45) has a single
// threshold, so no ambiguous band.
func euThreshold(value *int64, cpv string, t EUThreshold) string {
	if value == nil {
		return ""
	}
	v := *value
	if strings.HasPrefix(cpv, "45") { // works
		switch {
		case v < t.WorksMinor:
			return "below_eu"
		case v > t.WorksMinor:
			return "above_eu"
		default:
			return "" // exactly on the line — don't claim either
		}
	}
	// supplies/services
	switch {
	case v < t.SuppliesCentralMinor:
		return "below_eu"
	case v > t.SuppliesSubCentralMinor:
		return "above_eu"
	default:
		return "" // between central and sub-central: buyer-type ambiguous → no badge
	}
}

// EUThresholdBand exposes the pure euThreshold classifier to adapters (the
// ConnectRPC handler's tenderResultToProto), applying this service's injected
// EUThreshold config. Kept a thin method so the config stays private to the
// service and every result path classifies against the same knobs.
func (s *Service) EUThresholdBand(value *int64, cpv string) string {
	return euThreshold(value, cpv, s.cfg.EU)
}

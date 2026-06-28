// Package health implements the readiness use case at the core of the service.
// It depends only on the Probe port; adapters provide concrete probes.
package health

import "context"

// Probe is the port a readiness check implements. Name identifies the probe in
// the readiness report; Check returns nil when healthy or an error describing
// the failure.
type Probe interface {
	Name() string
	Check(ctx context.Context) error
}

// Status is the aggregated readiness result. OK is true only when every probe
// passed; Checks maps each failing probe's name to its error message.
type Status struct {
	OK     bool
	Checks map[string]string
}

// Service runs the registered probes to answer readiness queries.
type Service struct {
	probes []Probe
}

// New builds a Service over the given probes.
func New(probes ...Probe) *Service {
	return &Service{probes: probes}
}

// Ready runs every probe and aggregates the result. With no probes the service
// is ready by definition.
func (s *Service) Ready(ctx context.Context) Status {
	status := Status{OK: true, Checks: map[string]string{}}
	for _, p := range s.probes {
		if err := p.Check(ctx); err != nil {
			status.OK = false
			status.Checks[p.Name()] = err.Error()
		}
	}
	return status
}

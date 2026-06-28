// Package probe holds driven adapters implementing the health.Probe port.
package probe

import "context"

// Ready is a trivial readiness probe: a stateless service is ready as soon as
// it can serve. Future probes (datastore ping, upstream reachability) live here.
type Ready struct{}

// NewReady returns a Ready probe.
func NewReady() Ready { return Ready{} }

// Name identifies the probe in the readiness report.
func (Ready) Name() string { return "service" }

// Check always succeeds.
func (Ready) Check(context.Context) error { return nil }

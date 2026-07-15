// Package source holds one Source implementation per external tender
// provider (e.g. TED, a national procurement portal), plus the registry
// that assembles them for ingestion.Service.
//
// To add a provider:
//  1. Create a new package under internal/adapter/source/<name>/ implementing
//     ingestion.Source (Name() string, Fetch(ctx) ([]tender.Tender, error)).
//  2. Map the provider's native fields onto go-services/tender.Tender inside
//     Fetch — including its status text onto tender.Status (StatusUnknown if
//     it doesn't map cleanly).
//  3. Register an instance of it in registry.go's NewRegistry.
//
// ingestion.Service.RunOnce runs every registered provider concurrently and
// isolates each one's failures — no other wiring is required.
package source

package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
)

type AgentPricingRepo struct{ db *pg.DB }

func NewAgentPricingRepo(db *pg.DB) *AgentPricingRepo { return &AgentPricingRepo{db: db} }

var _ credits.PricingSource = (*AgentPricingRepo)(nil)

// FindByAgentType returns an agent type's per-token cost. A missing row comes
// back as pg.ErrNoRows, which credits.Service reads as "no pricing configured"
// and answers with its 1:1 fallback.
func (r *AgentPricingRepo) FindByAgentType(ctx context.Context, agentType string) (credits.Pricing, error) {
	var row DBAgentPricing
	if err := r.db.Select().From(AgentPricing).Where(APricingAgentType.Eq(agentType)).One(ctx, &row); err != nil {
		return credits.Pricing{}, err
	}
	return credits.Pricing{InputCost: row.InputCost, OutputCost: row.OutputCost}, nil
}

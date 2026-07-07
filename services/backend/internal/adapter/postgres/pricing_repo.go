package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
)

type AgentPricingRepo struct{ db *pg.DB }

func NewAgentPricingRepo(db *pg.DB) *AgentPricingRepo { return &AgentPricingRepo{db: db} }

func (r *AgentPricingRepo) FindByAgentType(ctx context.Context, agentType string) (DBAgentPricing, error) {
	var row DBAgentPricing
	err := r.db.Select().From(AgentPricing).Where(APricingAgentType.Eq(agentType)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return row, err
	}
	return row, err
}

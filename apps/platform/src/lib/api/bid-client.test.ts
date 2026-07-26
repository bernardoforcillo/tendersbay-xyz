import { describe, expect, it } from 'vitest';
import { bidClient } from './client';

describe('bidClient', () => {
  it('exposes every BidService RPC', () => {
    for (const rpc of [
      'addBid',
      'listBids',
      'getBid',
      'setGoNoGo',
      'advanceStage',
      'recordOutcome',
      'listChecklistItems',
      'upsertChecklistAnswer',
      'removeBid',
    ] as const) {
      expect(typeof (bidClient as Record<string, unknown>)[rpc]).toBe('function');
    }
  });
});

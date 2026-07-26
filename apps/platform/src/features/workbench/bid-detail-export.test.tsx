import { describe, expect, it } from 'vitest';
import { BidDetailPage } from '~/features/workbench';

describe('workbench feature barrel', () => {
  it('exports BidDetailPage', () => {
    expect(typeof BidDetailPage).toBe('function');
  });
});

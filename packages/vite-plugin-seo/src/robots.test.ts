import { describe, expect, it } from 'vitest';
import { buildRobots } from './robots';

describe('buildRobots', () => {
  it('lists user-agent, allow, and the sitemap url', () => {
    const txt = buildRobots({ hostname: 'https://tendersbay.xyz' });
    expect(txt).toContain('User-agent: *');
    expect(txt).toContain('Allow: /');
    expect(txt).toContain('Sitemap: https://tendersbay.xyz/sitemap.xml');
  });
});

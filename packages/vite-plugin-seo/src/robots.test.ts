import { describe, expect, it } from 'vitest';
import { buildRobots, DEFAULT_AI_CRAWLERS } from './robots';

describe('buildRobots', () => {
  it('lists the generic user-agent, allow, and the sitemap url', () => {
    const txt = buildRobots({ hostname: 'https://tendersbay.xyz' });
    expect(txt).toContain('User-agent: *');
    expect(txt).toContain('Allow: /');
    expect(txt).toContain('Sitemap: https://tendersbay.xyz/sitemap.xml');
    expect(txt).toContain('Sitemap: https://tendersbay.xyz/sitemap-tenders.xml');
  });

  it('explicitly allows every default AI crawler', () => {
    const txt = buildRobots({ hostname: 'https://tendersbay.xyz' });
    for (const agent of DEFAULT_AI_CRAWLERS) {
      expect(txt).toContain(`User-agent: ${agent}`);
    }
    // Curated, not blocked: no Disallow directives anywhere.
    expect(txt).not.toContain('Disallow');
    // Each AI agent gets its own Allow: / block -> one Allow per block + generic.
    expect(txt.match(/Allow: \//g)).toHaveLength(DEFAULT_AI_CRAWLERS.length + 1);
  });

  it('mentions the llms.txt overview for AI crawlers', () => {
    const txt = buildRobots({ hostname: 'https://tendersbay.xyz' });
    expect(txt).toContain('https://tendersbay.xyz/llms.txt');
  });

  it('honours a custom aiCrawlers list', () => {
    const txt = buildRobots({ hostname: 'https://tendersbay.xyz', aiCrawlers: ['MyBot'] });
    expect(txt).toContain('User-agent: MyBot');
    expect(txt).not.toContain('User-agent: GPTBot');
  });
});

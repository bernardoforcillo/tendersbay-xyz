import { describe, expect, it } from 'vitest';
import { headTags } from './head';

const opts = {
  hostname: 'https://tendersbay.xyz',
  siteName: 'tendersbay',
  description: 'Find EU tenders',
  ogImage: '/og.png',
  organization: { name: 'tendersbay', url: 'https://tendersbay.xyz' },
};

describe('headTags', () => {
  it('includes description, og, twitter, and an absolutised image', () => {
    const tags = headTags(opts);
    expect(
      tags.some((t) => t.attrs?.name === 'description' && t.attrs?.content === 'Find EU tenders'),
    ).toBe(true);
    expect(
      tags.some(
        (t) =>
          t.attrs?.property === 'og:image' && t.attrs?.content === 'https://tendersbay.xyz/og.png',
      ),
    ).toBe(true);
    expect(tags.some((t) => t.attrs?.name === 'twitter:card')).toBe(true);
  });

  it('emits no canonical link and a json-ld Organization', () => {
    const tags = headTags(opts);
    expect(tags.some((t) => t.tag === 'link' && t.attrs?.rel === 'canonical')).toBe(false);
    const ld = tags.find((t) => t.tag === 'script');
    expect(ld?.children).toContain('"@type":"Organization"');
  });
});

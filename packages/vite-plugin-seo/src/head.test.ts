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

  it('uses the title option for og:title and twitter:title', () => {
    const tags = headTags({ ...opts, title: 'EU public tenders — tendersbay' });
    expect(
      tags.some(
        (t) =>
          t.attrs?.property === 'og:title' && t.attrs?.content === 'EU public tenders — tendersbay',
      ),
    ).toBe(true);
    expect(
      tags.some(
        (t) =>
          t.attrs?.name === 'twitter:title' &&
          t.attrs?.content === 'EU public tenders — tendersbay',
      ),
    ).toBe(true);
  });

  it('falls back to siteName for og:title and twitter:title when title is absent', () => {
    const tags = headTags(opts);
    expect(
      tags.some((t) => t.attrs?.property === 'og:title' && t.attrs?.content === 'tendersbay'),
    ).toBe(true);
    expect(
      tags.some((t) => t.attrs?.name === 'twitter:title' && t.attrs?.content === 'tendersbay'),
    ).toBe(true);
  });

  it('emits no canonical link and a json-ld Organization', () => {
    const tags = headTags(opts);
    expect(tags.some((t) => t.tag === 'link' && t.attrs?.rel === 'canonical')).toBe(false);
    const ld = tags.find((t) => t.tag === 'script');
    expect(ld?.children).toContain('"@type":"Organization"');
  });

  it('adds a Service node to the @graph when service is set, provider -> Organization', () => {
    const tags = headTags({
      ...opts,
      service: {
        name: 'tendersbay',
        description: 'AI agents that find, prepare and help SMEs win EU public tenders',
        serviceType: 'Public procurement tender discovery',
        areaServed: 'European Union',
      },
    });
    const ld = tags.find((t) => t.tag === 'script');
    expect(ld?.children).toContain('"@type":"Service"');
    expect(ld?.children).toContain('"serviceType":"Public procurement tender discovery"');
    expect(ld?.children).toContain('"areaServed":"European Union"');
    expect(ld?.children).toContain('"provider":{"@type":"Organization"');
  });

  it('omits the Service node when service is absent', () => {
    const ld = headTags(opts).find((t) => t.tag === 'script');
    expect(ld?.children).not.toContain('"@type":"Service"');
  });
});

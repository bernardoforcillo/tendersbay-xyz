import { screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AgentsSection } from './index';

const captureMock = vi.hoisted(() => vi.fn());

vi.mock('posthog-js/react', () => ({
  usePostHog: () => ({ capture: captureMock }),
}));

// Force the section "in view" so the scroll-depth analytics fire deterministically;
// spread preserves the rest of motion/react (e.g. useKineticReveal's primitives).
vi.mock('motion/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('motion/react')>();
  return { ...actual, useInView: () => true };
});

describe('AgentsSection', () => {
  beforeEach(() => captureMock.mockClear());

  it('renders the overnight hook on a bold teal surface', () => {
    const { container } = renderWithI18n(<AgentsSection />, 'en-ie');
    const section = container.querySelector('#agents');
    expect(section).not.toBeNull();
    expect(section?.className).toContain('bg-brand-700');
    // Open-loop headline + tools-vs-agents wedge lead.
    expect(screen.getByText(/while you slept/i)).toBeInTheDocument();
    expect(screen.getByText(/the whole tender/i)).toBeInTheDocument();
  });

  it('renders the three cards as a 02:14 → 05:30 → 07:00 timeline', () => {
    const { container } = renderWithI18n(<AgentsSection />, 'en-ie');
    const times = Array.from(container.querySelectorAll('time')).map((el) => el.textContent);
    expect(times).toEqual(['02:14', '05:30', '07:00']);
    expect(screen.getByText('It found the one that fits')).toBeInTheDocument();
    expect(screen.getByText('It built the paperwork')).toBeInTheDocument();
    expect(screen.getByText('You woke up in the running')).toBeInTheDocument();
  });

  it('captures agents_section_viewed once when the hook scrolls into view', () => {
    renderWithI18n(<AgentsSection />, 'en-ie');
    expect(captureMock).toHaveBeenCalledTimes(1);
    expect(captureMock).toHaveBeenCalledWith('agents_section_viewed', { location: 'agents' });
  });
});

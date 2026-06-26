import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AgentsSection } from './index';

describe('AgentsSection', () => {
  it('renders the three agents as a numbered pipeline on a dark surface', () => {
    const { container } = renderWithI18n(<AgentsSection />, 'en-ie');
    const section = container.querySelector('#agents');
    expect(section).not.toBeNull();
    expect(section?.className).toContain('bg-ink-900');
    expect(screen.getByText('01')).toBeInTheDocument();
    expect(screen.getByText('02')).toBeInTheDocument();
    expect(screen.getByText('03')).toBeInTheDocument();
    expect(screen.getByText('Find')).toBeInTheDocument();
    expect(screen.getByText('Prepare')).toBeInTheDocument();
    expect(screen.getByText('Win')).toBeInTheDocument();
  });
});

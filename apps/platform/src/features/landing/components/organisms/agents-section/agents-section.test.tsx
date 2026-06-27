import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AgentsSection } from './index';

describe('AgentsSection', () => {
  it('renders the agent team on a bold teal surface', () => {
    const { container } = renderWithI18n(<AgentsSection />, 'en-ie');
    const section = container.querySelector('#agents');
    expect(section).not.toBeNull();
    expect(section?.className).toContain('bg-brand-700');
    expect(screen.getByText('Find')).toBeInTheDocument();
    expect(screen.getByText('Prepare')).toBeInTheDocument();
    expect(screen.getByText('Win')).toBeInTheDocument();
  });
});

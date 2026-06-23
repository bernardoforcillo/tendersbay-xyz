import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AgentsSection } from './index';

describe('AgentsSection', () => {
  it('renders the three agents', () => {
    const { container } = renderWithI18n(<AgentsSection />, 'en-ie');
    expect(container.querySelector('#agents')).not.toBeNull();
    expect(screen.getByText('Find')).toBeInTheDocument();
    expect(screen.getByText('Prepare')).toBeInTheDocument();
    expect(screen.getByText('Win')).toBeInTheDocument();
  });
});

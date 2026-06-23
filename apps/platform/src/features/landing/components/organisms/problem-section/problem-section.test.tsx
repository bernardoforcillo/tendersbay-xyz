import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { ProblemSection } from './index';

describe('ProblemSection', () => {
  it('renders the heading and all three problem cards', () => {
    const { container } = renderWithI18n(<ProblemSection />, 'en-ie');
    expect(
      screen.getByRole('heading', { name: 'The tender game is stacked against SMEs.' }),
    ).toBeInTheDocument();
    expect(container.querySelector('#problem')).not.toBeNull();
    expect(screen.getByText('Scattered across 27 countries')).toBeInTheDocument();
    expect(screen.getByText('No dedicated bids team')).toBeInTheDocument();
  });
});

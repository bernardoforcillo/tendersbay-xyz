import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { ProblemSection } from './index';

describe('ProblemSection', () => {
  it('renders the heading and all three problem cards', () => {
    const { container } = renderWithI18n(<ProblemSection />, 'en-ie');
    expect(
      screen.getByRole('heading', { name: "The game was rigged to keep you out. It's working." }),
    ).toBeInTheDocument();
    expect(container.querySelector('#problem')).not.toBeNull();
    expect(screen.getByText('Buried across 27 countries')).toBeInTheDocument();
    expect(screen.getByText('No bid office, no shot')).toBeInTheDocument();
  });
});

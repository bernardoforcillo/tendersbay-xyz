import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { VisionSection } from './index';

describe('VisionSection', () => {
  it('renders the heading and the pre-launch note', () => {
    const { container } = renderWithI18n(<VisionSection />, 'en-ie');
    expect(container.querySelector('#vision')).not.toBeNull();
    expect(
      screen.getByRole('heading', { name: 'European tenders, open to everyone' }),
    ).toBeInTheDocument();
    expect(screen.getByText("We're building. Check back soon.")).toBeInTheDocument();
  });
});

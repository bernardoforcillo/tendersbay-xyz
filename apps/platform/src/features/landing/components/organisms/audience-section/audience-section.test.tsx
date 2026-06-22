import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AudienceSection } from './index';

describe('AudienceSection', () => {
  it('renders its heading', () => {
    renderWithI18n(<AudienceSection />, 'en-ie');
    expect(
      screen.getByRole('heading', { name: 'Built for SMEs and entrepreneurs' }),
    ).toBeInTheDocument();
  });
});

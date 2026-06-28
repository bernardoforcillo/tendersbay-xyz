import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AudienceSection } from './index';

describe('AudienceSection', () => {
  it('renders its heading and the three persona cards', () => {
    renderWithI18n(<AudienceSection />, 'en-ie');
    expect(
      screen.getByRole('heading', { name: 'One of these is you. You already know which.' }),
    ).toBeInTheDocument();
    expect(screen.getByText('You run the bids')).toBeInTheDocument();
    expect(screen.getByText('You own the number')).toBeInTheDocument();
    expect(screen.getByText('You multiply across clients')).toBeInTheDocument();
  });
});

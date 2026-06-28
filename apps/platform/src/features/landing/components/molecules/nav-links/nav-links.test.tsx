import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { NavLinks } from './index';

describe('NavLinks', () => {
  it('renders three anchor links to the page sections', () => {
    renderWithI18n(<NavLinks />, 'en-ie');
    expect(screen.getByRole('link', { name: 'The cost' })).toHaveAttribute('href', '#problem');
    expect(screen.getByRole('link', { name: 'The agents' })).toHaveAttribute('href', '#agents');
    expect(screen.getByRole('link', { name: 'Vision' })).toHaveAttribute('href', '#vision');
  });
});

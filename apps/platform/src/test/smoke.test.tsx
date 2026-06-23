import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';

describe('test harness', () => {
  it('renders a component through renderWithI18n', () => {
    renderWithI18n(<p>harness ready</p>);
    expect(screen.getByText('harness ready')).toBeInTheDocument();
  });
});

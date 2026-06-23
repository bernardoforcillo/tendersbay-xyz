import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Logo } from './index';

describe('Logo', () => {
  it('renders the tendersbay wordmark', () => {
    render(<Logo />);
    expect(screen.getByText('tenders')).toBeInTheDocument();
    expect(screen.getByText('bay')).toBeInTheDocument();
  });
});

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { SocialLinks } from './index';

describe('SocialLinks', () => {
  it('renders a labelled nav with the three social links', () => {
    render(<SocialLinks label="Follow tendersbay" />);
    expect(screen.getByRole('navigation', { name: 'Follow tendersbay' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'GitHub' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'LinkedIn' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'X' })).toBeInTheDocument();
  });
});

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Field } from './index';

describe('Field', () => {
  it('associates the label with the input', () => {
    render(<Field label="Email" placeholder="you@example.com" />);
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
  });

  it('shows the description when there is no error', () => {
    render(<Field label="Email" description="Work address preferred" />);
    expect(screen.getByText('Work address preferred')).toBeInTheDocument();
  });

  it('shows the error and marks the field invalid', () => {
    render(<Field label="Email" errorMessage="Required" />);
    expect(screen.getByText('Required')).toBeInTheDocument();
    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'true');
  });
});

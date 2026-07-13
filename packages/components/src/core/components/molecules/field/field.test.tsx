import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Form } from 'react-aria-components';
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

  it('surfaces native constraint validation when no errorMessage is given', async () => {
    const user = userEvent.setup();
    render(
      <Form>
        <Field label="Email" name="email" isRequired />
        <button type="submit">Save</button>
      </Form>,
    );
    await user.click(screen.getByRole('button', { name: 'Save' }));
    const input = screen.getByLabelText('Email');
    expect(input).toHaveAttribute('aria-invalid', 'true');
  });
});

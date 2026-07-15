import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ChoicePromptCard } from './index';

const message = {
  id: 'msg-1',
  role: 'choice_prompt' as const,
  content: 'Private or shared?',
  createdAt: new Date().toISOString(),
  choices: [
    { key: 'A', label: 'Private', description: '' },
    { key: 'B', label: 'Shared', description: 'Visible to the workspace' },
  ],
};

describe('ChoicePromptCard', () => {
  it('renders the question and one button per option', () => {
    render(<ChoicePromptCard message={message} isPending onSubmit={vi.fn()} />);
    expect(screen.getByText('Private or shared?')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Private/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Shared/ })).toBeInTheDocument();
  });

  it('calls onSubmit with the selected key when pending', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<ChoicePromptCard message={message} isPending onSubmit={onSubmit} />);
    await user.click(screen.getByRole('button', { name: /Shared/ }));
    expect(onSubmit).toHaveBeenCalledWith('B', undefined);
  });

  it('disables the option buttons when not pending (already answered)', () => {
    render(<ChoicePromptCard message={message} isPending={false} onSubmit={vi.fn()} />);
    expect(screen.getByRole('button', { name: /Private/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /Shared/ })).toBeDisabled();
  });
});

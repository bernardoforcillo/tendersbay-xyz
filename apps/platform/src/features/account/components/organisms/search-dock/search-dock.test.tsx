import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { SearchDock } from './index';

function ControlledDock({ onSubmit }: { onSubmit: (value: string) => void }) {
  const [value, setValue] = useState('');
  return (
    <SearchDock mode="search" value={value} onChange={setValue} onSubmit={() => onSubmit(value)} />
  );
}

describe('SearchDock', () => {
  describe('decorative mode (no handlers)', () => {
    it('renders the first localized example as the looping placeholder', () => {
      renderWithI18n(<SearchDock />, 'en-ie');
      expect(screen.getByText('Public school renovations')).toBeInTheDocument();
    });

    it('exposes a focusable, disabled, grayscale control labelled for assistive tech', () => {
      renderWithI18n(<SearchDock />, 'en-ie');
      const control = screen.getByRole('button', { name: 'Search' });
      expect(control).toHaveAttribute('aria-disabled', 'true');
      expect(control.className).toContain('grayscale');
      expect(control.className).toContain('cursor-default');
      control.focus();
      expect(control).toHaveFocus();
    });

    it('renders no input, since the dock is purely decorative', () => {
      renderWithI18n(<SearchDock />, 'en-ie');
      expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    });
  });

  describe('pressable decorative mode (onPress only)', () => {
    it('drops aria-disabled and grayscale, uses cursor-pointer, and calls onPress', async () => {
      const user = userEvent.setup();
      const onPress = vi.fn();
      renderWithI18n(<SearchDock onPress={onPress} />, 'en-ie');

      const control = screen.getByRole('button', { name: 'Search' });
      expect(control).not.toHaveAttribute('aria-disabled');
      expect(control.className).not.toContain('grayscale');
      expect(control.className).toContain('cursor-pointer');

      await user.click(control);
      expect(onPress).toHaveBeenCalledTimes(1);
    });
  });

  describe('functional mode (onSubmit + mode=search)', () => {
    it('renders a real, labelled text input showing the rotating example as placeholder', () => {
      renderWithI18n(
        <SearchDock mode="search" value="" onChange={() => {}} onSubmit={() => {}} />,
        'en-ie',
      );
      const input = screen.getByRole('textbox', { name: 'Search' });
      expect(input).toHaveAttribute('placeholder', 'Public school renovations');
      // The animated overlay duplicates the same copy while empty and unfocused.
      expect(screen.getByText('Public school renovations')).toBeInTheDocument();
    });

    it('has no aria-disabled and no grayscale on the form pill', () => {
      renderWithI18n(
        <SearchDock mode="search" value="" onChange={() => {}} onSubmit={() => {}} />,
        'en-ie',
      );
      const form = screen.getByRole('textbox', { name: 'Search' }).closest('form');
      expect(form).not.toBeNull();
      expect(form).not.toHaveAttribute('aria-disabled');
      expect(form?.className).not.toContain('grayscale');
    });

    it('hides the animated placeholder once the input is focused', async () => {
      const user = userEvent.setup();
      renderWithI18n(
        <SearchDock mode="search" value="" onChange={() => {}} onSubmit={() => {}} />,
        'en-ie',
      );
      const input = screen.getByRole('textbox', { name: 'Search' });
      await user.click(input);
      expect(screen.queryByText('Public school renovations')).not.toBeInTheDocument();
    });

    it('typing updates the controlled value and submitting calls onSubmit with the query', async () => {
      const user = userEvent.setup();
      const handleSubmit = vi.fn();
      renderWithI18n(<ControlledDock onSubmit={handleSubmit} />, 'en-ie');

      const input = screen.getByRole('textbox', { name: 'Search' });
      await user.type(input, 'roads');
      expect(input).toHaveValue('roads');

      await user.click(screen.getByRole('button', { name: 'Search or ask…' }));
      expect(handleSubmit).toHaveBeenCalledWith('roads');
    });

    it('submits on Enter inside the input', async () => {
      const user = userEvent.setup();
      const handleSubmit = vi.fn();
      renderWithI18n(<ControlledDock onSubmit={handleSubmit} />, 'en-ie');

      const input = screen.getByRole('textbox', { name: 'Search' });
      await user.type(input, 'bridges{Enter}');
      expect(handleSubmit).toHaveBeenCalledWith('bridges');
    });
  });
});

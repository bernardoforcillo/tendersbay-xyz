import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { Select } from './index';

describe('Select', () => {
  it('associates the label and lets a user choose an option', async () => {
    const user = userEvent.setup();
    render(
      <Select label="Country" defaultValue="ie">
        <option value="ie">Ireland</option>
        <option value="fr">France</option>
      </Select>,
    );
    const select = screen.getByLabelText('Country') as HTMLSelectElement;
    await user.selectOptions(select, 'fr');
    expect(select.value).toBe('fr');
  });

  it('applies the kit input classes and merges a consumer className', () => {
    render(
      <Select label="Country" className="w-40">
        <option value="ie">Ireland</option>
      </Select>,
    );
    const select = screen.getByLabelText('Country');
    expect(select.className).toContain('rounded-xl');
    expect(select.className).toContain('w-40');
  });
});

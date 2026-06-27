import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AssuranceSection } from './index';

describe('AssuranceSection', () => {
  it('renders the heading and the objection cards', () => {
    const { container } = renderWithI18n(<AssuranceSection />, 'en-ie');
    expect(
      screen.getByRole('heading', { name: 'Ask the questions that usually kill the deal.' }),
    ).toBeInTheDocument();
    expect(container.querySelector('#assurance')).not.toBeNull();
    expect(screen.getByText('“Does my data train your AI?”')).toBeInTheDocument();
    expect(screen.getByText('“Does it fit what I already use?”')).toBeInTheDocument();
  });
});

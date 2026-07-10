import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it } from 'vitest';
import { useSidebarStore } from '~/store/sidebar';
import { PageHeader } from './index';

describe('PageHeader', () => {
  beforeEach(() => {
    useSidebarStore.setState({ collapsed: false, drawerOpen: false });
  });

  it('renders the title as a heading', () => {
    render(<PageHeader title="Members" />);
    expect(screen.getByRole('heading', { name: 'Members' })).toBeInTheDocument();
  });

  it('renders no heading for a toggle-only bar', () => {
    render(<PageHeader />);
    expect(screen.queryByRole('heading')).toBeNull();
  });

  it('collapses the rail when the desktop toggle is pressed', async () => {
    const user = userEvent.setup();
    render(<PageHeader title="Members" />);
    await user.click(screen.getByRole('button', { name: 'Collapse sidebar' }));
    expect(useSidebarStore.getState().collapsed).toBe(true);
  });

  it('opens the drawer when the mobile toggle is pressed', async () => {
    const user = userEvent.setup();
    render(<PageHeader title="Members" />);
    await user.click(screen.getByRole('button', { name: 'Open navigation' }));
    expect(useSidebarStore.getState().drawerOpen).toBe(true);
  });

  it('renders subtitle, actions and children slots', () => {
    render(
      <PageHeader subtitle="@acme" actions={<button type="button">New</button>} title="Workspace">
        <div>tabs</div>
      </PageHeader>,
    );
    expect(screen.getByText('@acme')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New' })).toBeInTheDocument();
    expect(screen.getByText('tabs')).toBeInTheDocument();
  });
});

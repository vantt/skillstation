import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../../api/client';
import type { CheckoutDiscardPreview, CheckoutList, CheckoutView } from '../../api/types';

const previewCheckoutDiscard = vi.fn();
const confirmCheckoutDiscard = vi.fn();

vi.mock('../../api/queries', async (orig) => {
  const actual = await orig<typeof import('../../api/queries')>();
  return {
    ...actual,
    previewCheckoutDiscard: (...args: unknown[]) => previewCheckoutDiscard(...args),
    confirmCheckoutDiscard: (...args: unknown[]) => confirmCheckoutDiscard(...args),
  };
});

import { CheckoutsScreen } from './CheckoutsScreen';

function checkout(over: Partial<CheckoutView> = {}): CheckoutView {
  return {
    skill_id: 'pdf',
    project_root: '/home/me/app',
    folder: '/home/me/app/.agents/skills/pdf',
    pointer: '/home/me/app/.claude/skills/pdf/SKILL.md',
    started_at: '2026-10-20T09:30:00Z',
    state: 'open',
    new_skill: false,
    changed_files: 2,
    official_moved: false,
    ...over,
  };
}

function preview(over: Partial<CheckoutDiscardPreview> = {}): CheckoutDiscardPreview {
  return {
    status: 'action_required',
    summary: 'Discarding deletes the folder.',
    checkout: checkout(),
    unsaved: ['SKILL.md', 'scripts/run.sh'],
    left_out: ['.env'],
    digest: 'sha256:one',
    ...over,
  };
}

let queryClient: QueryClient;

function renderScreen(checkouts: CheckoutView[]) {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  const list: CheckoutList = { status: 'ok', summary: `${checkouts.length} checkouts open.`, checkouts };
  queryClient.setQueryData(['checkouts'], list);
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CheckoutsScreen />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('CheckoutsScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('says there are no checkouts when the list is empty', () => {
    renderScreen([]);
    expect(screen.getByText('No open checkouts.')).toBeInTheDocument();
  });

  it('lists each checkout with its skill, project, start date and changed files', () => {
    renderScreen([
      checkout({ official_moved: true }),
      checkout({ skill_id: 'brand-new', project_root: '/home/me/site', new_skill: true, changed_files: 1 }),
    ]);

    const rows = screen.getAllByRole('listitem');
    const first = within(rows[0]!);
    expect(first.getByRole('link', { name: 'pdf' })).toHaveAttribute('href', '/skills/pdf');
    expect(first.getByText('/home/me/app')).toBeInTheDocument();
    expect(first.getByText('Started 2026-10-20')).toBeInTheDocument();
    expect(first.getByText('2 files changed since it started')).toBeInTheDocument();
    expect(first.getByText('The hub changed since; a commit will merge.')).toBeInTheDocument();

    const second = within(rows[1]!);
    expect(second.queryByRole('link')).not.toBeInTheDocument();
    expect(second.getByText('brand-new')).toBeInTheDocument();
    expect(second.getByText('New skill')).toBeInTheDocument();
    expect(second.getByText('1 file changed since it started')).toBeInTheDocument();
    expect(second.queryByText('The hub changed since; a commit will merge.')).not.toBeInTheDocument();
  });

  it('names by file what discarding loses, then discards with the preview digest and refreshes the list', async () => {
    previewCheckoutDiscard.mockResolvedValue(preview());
    confirmCheckoutDiscard.mockResolvedValue({ status: 'applied', summary: 'Discarded.' });
    // The list the hub returns once the checkout is gone.
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ status: 'ok', summary: '0 checkouts open.', checkouts: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    renderScreen([checkout()]);

    fireEvent.click(screen.getByRole('button', { name: 'Discard checkout pdf' }));
    const dialog = await screen.findByRole('alertdialog');
    expect(await within(dialog).findByText('SKILL.md')).toBeInTheDocument();
    expect(previewCheckoutDiscard).toHaveBeenCalledWith('pdf', '/home/me/app');
    expect(within(dialog).getByText('scripts/run.sh')).toBeInTheDocument();
    expect(within(dialog).getByText('.env')).toBeInTheDocument();
    expect(within(dialog).getByText('The hub keeps its own version of pdf.')).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole('button', { name: 'Discard' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(confirmCheckoutDiscard).toHaveBeenCalledWith('pdf', '/home/me/app', 'sha256:one');
    expect(screen.getByText('Discarded the checkout of pdf in /home/me/app.')).toBeInTheDocument();
    expect(await screen.findByText('No open checkouts.')).toBeInTheDocument();
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/checkouts');
  });

  it('tells the person the folder changed and shows the new preview when the discard is stale', async () => {
    previewCheckoutDiscard
      .mockResolvedValueOnce(preview())
      .mockResolvedValueOnce(preview({ unsaved: ['SKILL.md', 'notes.md'], left_out: [], digest: 'sha256:two' }));
    confirmCheckoutDiscard
      .mockRejectedValueOnce(
        new ApiError(409, {
          code: 'stale_proposal',
          render: { ERROR: 'The proposal is stale.', WHY: 'the draft folder changed', FIX: 'Open it again.' },
        }),
      )
      .mockResolvedValueOnce({ status: 'applied', summary: 'Discarded.' });
    renderScreen([checkout()]);

    fireEvent.click(screen.getByRole('button', { name: 'Discard checkout pdf' }));
    const dialog = await screen.findByRole('alertdialog');
    await within(dialog).findByText('.env');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Discard' }));

    expect(await within(dialog).findByText(/The checkout folder changed since you looked/)).toBeInTheDocument();
    expect(await within(dialog).findByText('notes.md')).toBeInTheDocument();
    expect(within(dialog).queryByText('.env')).not.toBeInTheDocument();
    expect(previewCheckoutDiscard).toHaveBeenCalledTimes(2);
    expect(screen.getByRole('alertdialog')).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole('button', { name: 'Discard' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(confirmCheckoutDiscard).toHaveBeenLastCalledWith('pdf', '/home/me/app', 'sha256:two');
  });

  it('offers to remove the record when the project no longer has the checkout', async () => {
    const gone = checkout({ state: 'folder_gone', changed_files: 0 });
    previewCheckoutDiscard.mockResolvedValue(preview({ checkout: gone, unsaved: [], left_out: [], digest: 'sha256:gone' }));
    confirmCheckoutDiscard.mockResolvedValue({ status: 'applied', summary: 'Discarded.' });
    renderScreen([gone]);

    expect(screen.getByText('The project no longer has this checkout.')).toBeInTheDocument();
    expect(screen.queryByText(/files? changed since it started/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Remove pdf' }));

    const dialog = await screen.findByRole('alertdialog');
    expect(await within(dialog).findByText(/Removing deletes only the hub’s record/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Remove record' }));
    await waitFor(() => expect(confirmCheckoutDiscard).toHaveBeenCalledWith('pdf', '/home/me/app', 'sha256:gone'));
  });

  it('shows why the preview failed and keeps Discard off', async () => {
    previewCheckoutDiscard.mockRejectedValue(
      new ApiError(404, {
        code: 'invalid_request',
        render: { ERROR: 'The hub has no record of that draft any more.', WHY: '', FIX: 'Reload the page.' },
      }),
    );
    renderScreen([checkout()]);

    fireEvent.click(screen.getByRole('button', { name: 'Discard checkout pdf' }));
    const dialog = await screen.findByRole('alertdialog');
    expect(await within(dialog).findByText(/The hub has no record of that draft any more/)).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Discard' })).toBeDisabled();
    expect(confirmCheckoutDiscard).not.toHaveBeenCalled();
  });
});

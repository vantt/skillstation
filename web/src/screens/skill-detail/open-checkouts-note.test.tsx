import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';
import type { CheckoutList, CheckoutView } from '../../api/types';
import { OpenCheckoutsNote } from './OpenCheckoutsNote';

function checkout(over: Partial<CheckoutView>): CheckoutView {
  return {
    skill_id: 'pdf',
    project_root: '/home/me/app',
    folder: '/home/me/app/.agents/skills/pdf',
    started_at: '2026-10-20T09:30:00Z',
    state: 'open',
    new_skill: false,
    changed_files: 0,
    official_moved: false,
    ...over,
  };
}

function renderNote(skillId: string, checkouts: CheckoutView[]) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  const list: CheckoutList = { status: 'ok', summary: '', checkouts };
  queryClient.setQueryData(['checkouts'], list);
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <OpenCheckoutsNote skillId={skillId} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('OpenCheckoutsNote', () => {
  it('names each project that has this skill checked out and links to the Checkouts page', () => {
    renderNote('pdf', [
      checkout({}),
      checkout({ skill_id: 'other', project_root: '/home/me/other' }),
      checkout({ project_root: '/home/me/gone', state: 'folder_gone' }),
    ]);

    const link = screen.getByRole('link', { name: 'Open checkout in /home/me/app since 2026-10-20' });
    expect(link).toHaveAttribute('href', '/checkouts');
    expect(screen.getAllByRole('link')).toHaveLength(1);
  });

  it('shows nothing when the skill has no open checkout', () => {
    const { container } = renderNote('pdf', [checkout({ skill_id: 'other' })]);
    expect(container).toBeEmptyDOMElement();
  });
});

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ReviewQueueItem } from '../../api/types';

let items: ReviewQueueItem[] = [];
const discardSubmission = vi.fn(async (id: string) => {
  void id;
});

vi.mock('../../api/queries', () => ({
  useReviewQueue: () => ({ data: { items }, isLoading: false, error: null }),
  discardSubmission: (id: string) => discardSubmission(id),
}));

import { PendingSubmissionsNote } from './PendingSubmissionsNote';

function submission(skillId: string, id: string, since: string): ReviewQueueItem {
  return { kind: 'submission', skill_id: skillId, title: skillId, reason: 'a change submitted for review', since, origin: 'Submitted from laptop', submission_id: id };
}

function renderNote(skillId = 'notes') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <PendingSubmissionsNote skillId={skillId} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('PendingSubmissionsNote', () => {
  beforeEach(() => {
    discardSubmission.mockClear();
  });

  it('shows nothing when no update of this skill waits', () => {
    items = [submission('other', 'SUB-01M6NB8W', '2026-11-05'), { kind: 'draft', skill_id: 'notes', title: 'Notes', reason: 'new draft' }];
    const { container } = renderNote();
    expect(container).toBeEmptyDOMElement();
  });

  it('says an update waits and links to its review', () => {
    items = [submission('notes', 'SUB-01M6NB8W', '2026-11-05')];
    renderNote();
    expect(screen.getByRole('status')).toHaveTextContent(
      'An update to this skill is waiting for review: SUB-01M6NB8W, sent 2026-11-05 from laptop.',
    );
    expect(screen.getByRole('link', { name: 'Review it →' })).toHaveAttribute('href', '/review/submissions/SUB-01M6NB8W');
    expect(screen.queryByRole('button', { name: 'Discard older' })).not.toBeInTheDocument();
  });

  it('points to the newest of several and discards the older ones after confirmation', async () => {
    items = [
      submission('notes', 'SUB-01M6DMW0', '2026-11-02'),
      submission('notes', 'SUB-01M6NB8W', '2026-11-05'),
      submission('notes', 'SUB-01M6NAX3', '2026-11-05'),
    ];
    renderNote();
    expect(screen.getByRole('status')).toHaveTextContent('SUB-01M6NB8W');
    expect(screen.getByRole('status')).toHaveTextContent('2 older submissions are also waiting; the newest replaces them.');
    expect(screen.getByRole('link', { name: 'Review it →' })).toHaveAttribute('href', '/review/submissions/SUB-01M6NB8W');

    fireEvent.click(screen.getByRole('button', { name: 'Discard older' }));
    expect(discardSubmission).not.toHaveBeenCalled();
    expect(screen.getByText('Discard 2 older submissions of notes? The newest one stays waiting for review.')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: 'Discard older' }).at(-1)!);

    await waitFor(() => expect(discardSubmission).toHaveBeenCalledTimes(2));
    expect(discardSubmission.mock.calls.map(([id]) => id)).toEqual(['SUB-01M6NAX3', 'SUB-01M6DMW0']);
  });
});

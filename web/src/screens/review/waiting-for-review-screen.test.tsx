import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import type { ReviewQueueItem } from '../../api/types';

let items: ReviewQueueItem[] = [];

vi.mock('../../api/queries', () => ({
  useReviewQueue: () => ({ data: { items }, isLoading: false, error: null }),
}));

import { WaitingForReviewScreen } from './WaitingForReviewScreen';

function renderScreen() {
  return render(
    <MemoryRouter>
      <WaitingForReviewScreen />
    </MemoryRouter>,
  );
}

describe('WaitingForReviewScreen', () => {
  it('lists drafts and changed third-party skills with a link to each full review', () => {
    items = [
      { kind: 'content_changed', skill_id: 'pdf', title: 'PDF', reason: 'content changed since it was approved', since: '2026-10-14', origin: 'github.com/anthropics/skills' },
      { kind: 'draft', skill_id: 'team-release', title: 'Team release', reason: 'new draft', since: '2026-10-14' },
      { kind: 'submission', skill_id: 'release-notes', title: 'release-notes', reason: 'a new skill submitted for review', since: '2026-10-14', origin: 'Submitted from machine-a', submission_id: 'SUB-01JA2' },
    ];
    renderScreen();
    expect(screen.getByRole('heading', { name: 'Waiting for review (3)' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'PDF' })).toHaveAttribute('href', '/skills/pdf/review-content');
    expect(screen.getByText('Content changed · needs approval')).toBeInTheDocument();
    expect(screen.getByText('New draft')).toBeInTheDocument();
    expect(screen.getByText('github.com/anthropics/skills')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'release-notes' })).toHaveAttribute('href', '/review/submissions/SUB-01JA2');
    expect(screen.getByText('Submitted · needs approval')).toBeInTheDocument();
    expect(screen.getByText('Submitted from machine-a')).toBeInTheDocument();
  });

  it('marks older submissions of a skill that a newer one replaces', () => {
    items = [
      { kind: 'submission', skill_id: 'notes', title: 'Notes', reason: 'a change submitted for review', since: '2026-11-05', submission_id: 'SUB-01M6NB8W' },
      { kind: 'submission', skill_id: 'notes', title: 'Notes', reason: 'a change submitted for review', since: '2026-11-02', submission_id: 'SUB-01M6DMW0' },
    ];
    renderScreen();
    expect(screen.getAllByText('Older · a newer submission is waiting')).toHaveLength(1);
    const olderRow = screen.getAllByRole('listitem').find((row) => row.querySelector('a')?.getAttribute('href') === '/review/submissions/SUB-01M6DMW0');
    expect(olderRow).toHaveTextContent('Older · a newer submission is waiting');
  });

  it('says so when nothing waits', () => {
    items = [];
    renderScreen();
    expect(screen.getByText('Nothing is waiting for review')).toBeInTheDocument();
  });
});

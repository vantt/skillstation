import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../../api/client';
import type { SkillReviewContent, SubmissionReview } from '../../api/types';

const previewSkillApproval = vi.fn();
const confirmSkillMutation = vi.fn();
const previewSubmissionApproval = vi.fn();
const discardSubmission = vi.fn();
let review: SkillReviewContent;
let submission: SubmissionReview;
let submissionError: unknown = null;
let queueItems: { kind: string; skill_id: string; submission_id?: string }[] = [];

vi.mock('../../api/queries', () => ({
  useSkillReviewContent: (id: string) => ({ data: id ? review : undefined, isLoading: false, error: null }),
  useSubmissionReview: (id: string) => ({ data: id && !submissionError ? submission : undefined, isLoading: false, error: submissionError }),
  useReviewQueue: () => ({ data: { items: queueItems } }),
  previewSubmissionApproval: (...args: unknown[]) => previewSubmissionApproval(...args),
  discardSubmission: (...args: unknown[]) => discardSubmission(...args),
  previewSkillApproval: (...args: unknown[]) => previewSkillApproval(...args),
  confirmSkillMutation: (...args: unknown[]) => confirmSkillMutation(...args),
}));

import { FullReviewScreen } from './FullReviewScreen';

function makeReview(over: Partial<SkillReviewContent> = {}): SkillReviewContent {
  return {
    schema_version: '1',
    status: 'ok',
    summary: '',
    items: [],
    suggested_actions: [],
    warnings: [],
    error: null,
    skill_id: 'pdf',
    state: 'active',
    title: 'PDF',
    license: 'Proprietary',
    origin: 'github.com/anthropics/skills@main',
    third_party: true,
    approved: false,
    content_digest: 'sha256:abc',
    baseline: 'approved',
    baseline_commit: '1234567890abcdef',
    files: [
      { path: 'SKILL.md', size_bytes: 10, binary: false, changed: true, change: 'M', added: 12, removed: 3, diff: '--- a/SKILL.md\n+++ b/SKILL.md\n@@ -1 +1 @@\n-a\n+b\n', truncated: false, content: 'b' },
      { path: 'scripts/fill.py', size_bytes: 10, binary: false, changed: true, change: 'M', added: 2, removed: 2, diff: '--- a/f\n+++ b/f\n@@ -1 +1 @@\n-x\n+y\n', truncated: false, content: 'y' },
      { path: 'references/a.md', size_bytes: 3, binary: false, changed: false, added: 0, removed: 0, truncated: false, content: 'a' },
    ],
    ...over,
  };
}

function makeSubmission(over: Partial<SubmissionReview> = {}): SubmissionReview {
  const base = makeReview({ skill_id: 'team-release', title: 'Team release', state: 'new', baseline: 'none', origin: 'Submitted from machine-a', baseline_commit: undefined });
  return {
    ...base,
    files: [
      { path: 'SKILL.md', size_bytes: 10, binary: false, changed: true, change: 'A', added: 5, removed: 0, diff: '--- /dev/null\n+++ b/SKILL.md\n@@ -0,0 +1 @@\n+x\n', truncated: false, content: 'x' },
    ],
    submission_id: 'SUB-01JA2',
    client_label: 'machine-a',
    submitted_at: '2026-10-14T10:00:00Z',
    note: '',
    case: 'new',
    conflicts: [],
    excluded: [{ path: '.env', reason: 'looks like a secret' }],
    ...over,
  };
}

function ReviewListStub() {
  return <div data-testid="review-list">{JSON.stringify((useLocation().state as { approved?: unknown } | null)?.approved)}</div>;
}

function renderScreen(entry = '/skills/pdf/review-content') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/skills/:id/review-content" element={<FullReviewScreen />} />
          <Route path="/review/submissions/:sid" element={<FullReviewScreen />} />
          <Route path="/review" element={<ReviewListStub />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function open(path: string) {
  const summary = screen.getByText(path).closest('summary')!;
  fireEvent.click(summary);
  // jsdom does not fire the toggle event itself.
  const details = summary.parentElement as HTMLDetailsElement;
  details.open = true;
  fireEvent(details, new Event('toggle'));
}

describe('FullReviewScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    review = makeReview();
    submission = makeSubmission();
    submissionError = null;
    queueItems = [];
  });

  it('shows the license, the origin, what the files are compared with, and the files in order', () => {
    renderScreen();
    expect(screen.getByText('License: Proprietary · Origin: github.com/anthropics/skills@main')).toBeInTheDocument();
    expect(screen.getByText(/Compared with the content you last approved \(commit 1234567\)/)).toBeInTheDocument();
    const changed = screen.getByRole('list', { name: 'Changed files' });
    expect(changed.textContent).toMatch(/SKILL\.md.*scripts\/fill\.py/s);
    expect(screen.getByText(/1 unchanged file/)).toBeInTheDocument();
  });

  it('keeps Approve disabled until every changed file was opened', async () => {
    previewSkillApproval.mockResolvedValue({ confirmation: { confirmation: { pins: { proposal_id: 'P', proposal_digest: 'D', base_version: 'B' } } } });
    confirmSkillMutation.mockResolvedValue({
      commit: { status: 'committed', hash: 'abcdef012345', reach: { this_hub_now: true, push_needed: false } },
    });
    renderScreen();
    const approve = screen.getByRole('button', { name: 'Approve' });
    expect(approve).toBeDisabled();
    expect(screen.getByTestId('approve-hint')).toHaveTextContent('open every changed file first: 2 left');

    open('SKILL.md');
    expect(approve).toBeDisabled();
    expect(screen.getByTestId('approve-hint')).toHaveTextContent('1 left');
    expect(screen.getByText('(opened)')).toBeInTheDocument();

    open('scripts/fill.py');
    expect(approve).toBeEnabled();
    expect(screen.queryByTestId('approve-hint')).not.toBeInTheDocument();

    fireEvent.click(approve);
    await waitFor(() => expect(confirmSkillMutation).toHaveBeenCalled());
    expect(previewSkillApproval).toHaveBeenCalledWith('pdf', 'sha256:abc');
    expect(confirmSkillMutation).toHaveBeenCalledWith('P', { proposalDigest: 'D', baseVersion: 'B' });
    expect(await screen.findByTestId('review-list')).toHaveTextContent('{"skillId":"pdf","title":"PDF","draft":false}');
  });

  it('opens every changed file at once so Approve unlocks, and collapses them again', () => {
    renderScreen();
    expect(screen.getByTestId('open-progress')).toHaveTextContent('0 of 2 changed files opened');
    fireEvent.click(screen.getByRole('button', { name: 'Select all and open' }));
    expect(screen.getByTestId('open-progress')).toHaveTextContent('2 of 2 changed files opened');
    expect(screen.getByRole('button', { name: 'Approve' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: 'Collapse all' }));
    expect(screen.getByRole('button', { name: 'Collapse all' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Approve' })).toBeEnabled();
  });

  it('has nothing to approve for a skill that is not from another repository', () => {
    review = makeReview({ third_party: false, state: 'draft', baseline: 'none' });
    renderScreen();
    expect(screen.queryByRole('button', { name: 'Approve' })).not.toBeInTheDocument();
    expect(screen.getByText(/nothing to approve/)).toBeInTheDocument();
  });

  it('reviews a submission: submitted-from origin, files, what was left out, and approves it with its commit message', async () => {
    previewSubmissionApproval.mockResolvedValue({
      default_message: 'Add team-release (submitted from machine-a), approve content',
      confirmation: { confirmation: { pins: { proposal_id: 'P', proposal_digest: 'D', base_version: 'B' } } },
    });
    confirmSkillMutation.mockResolvedValue({
      commit: { status: 'committed', hash: 'abcdef012345', reach: { this_hub_now: true, push_needed: false } },
    });
    renderScreen('/review/submissions/SUB-01JA2');
    expect(screen.getByText('License: Proprietary · Origin: Submitted from machine-a')).toBeInTheDocument();
    expect(screen.getByText(/A new draft: every file is new/)).toBeInTheDocument();
    expect(screen.getByText(/Left out by the submitting machine: \.env \(looks like a secret\)/)).toBeInTheDocument();
    const approve = screen.getByRole('button', { name: 'Approve' });
    expect(approve).toBeDisabled();
    open('SKILL.md');
    expect(approve).toBeEnabled();
    fireEvent.click(approve);
    await waitFor(() => expect(confirmSkillMutation).toHaveBeenCalled());
    expect(previewSubmissionApproval).toHaveBeenCalledWith('SUB-01JA2', 'sha256:abc');
    expect(confirmSkillMutation).toHaveBeenCalledWith(
      'P',
      { proposalDigest: 'D', baseVersion: 'B' },
      'Add team-release (submitted from machine-a), approve content',
      'SUB-01JA2',
    );
    expect(await screen.findByTestId('review-list')).toHaveTextContent('{"skillId":"team-release","title":"Team release","draft":true}');
  });

  it('shows the sender\'s note as written, line breaks kept, and nothing when there is none', () => {
    submission = makeSubmission({ note: 'It skipped the tag step.\nPlease check the release flow.' });
    const { unmount } = renderScreen('/review/submissions/SUB-01JA2');
    const note = screen.getByRole('region', { name: 'Note from the sender' });
    expect(note).toHaveTextContent('It skipped the tag step.');
    expect(note).toHaveTextContent('Please check the release flow.');
    unmount();
    submission = makeSubmission({ note: '' });
    renderScreen('/review/submissions/SUB-01JA2');
    expect(screen.queryByRole('region', { name: 'Note from the sender' })).not.toBeInTheDocument();
  });

  it('keeps Approve off while files need a decision on the hub machine, and says how', () => {
    submission = makeSubmission({ case: 'merge', conflicts: ['SKILL.md'] });
    renderScreen('/review/submissions/SUB-01JA2');
    open('SKILL.md');
    expect(screen.getByRole('button', { name: 'Approve' })).toBeDisabled();
    expect(screen.getByRole('alert')).toHaveTextContent('skillstation skill review SUB-01JA2 --accept <path>=hub');
  });

  it('tells an older submission from the newest one, and links to the newest', () => {
    queueItems = [
      { kind: 'submission', skill_id: 'team-release', submission_id: 'SUB-01JA2' },
      { kind: 'submission', skill_id: 'team-release', submission_id: 'SUB-01JA9' },
    ];
    const { unmount } = renderScreen('/review/submissions/SUB-01JA2');
    expect(screen.getByRole('status').textContent).toContain('older submission, sent 2026-10-14 10:00 UTC');
    expect(screen.getByRole('link', { name: /Review it/ }).getAttribute('href')).toBe('/review/submissions/SUB-01JA9');
    unmount();
    submission = makeSubmission({ submission_id: 'SUB-01JA9' });
    renderScreen('/review/submissions/SUB-01JA9');
    expect(screen.getByRole('status').textContent).toContain('newest submission of this skill');
    expect(screen.queryByRole('link', { name: /Review it/ })).toBeNull();
  });

  it('shows no order note once the submission it describes has left the queue', () => {
    queueItems = [{ kind: 'submission', skill_id: 'team-release', submission_id: 'SUB-01JA1' }];
    renderScreen('/review/submissions/SUB-01JA2');
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('says why a submission cannot be read instead of loading forever', () => {
    submissionError = new ApiError(400, {
      code: 'invalid_request',
      render: { ERROR: 'The request cannot be accepted.', WHY: 'routing.distinguish_from entries require a discriminator mapping', FIX: 'Fix what is reported in the folder.' },
    });
    renderScreen('/review/submissions/SUB-01JA2');
    expect(screen.getByRole('alert').textContent).toContain('require a discriminator mapping');
  });

  it('discards a submission only after a second click', async () => {
    discardSubmission.mockResolvedValue(undefined);
    renderScreen('/review/submissions/SUB-01JA2');
    fireEvent.click(screen.getByRole('button', { name: 'Discard' }));
    expect(discardSubmission).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Yes, discard' }));
    await waitFor(() => expect(discardSubmission).toHaveBeenCalledWith('SUB-01JA2'));
    expect(await screen.findByTestId('discard-result')).toHaveTextContent('Discarded.');
  });
});

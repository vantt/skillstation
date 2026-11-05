import { describe, expect, it } from 'vitest';
import type { ReviewQueueItem } from '../api/types';
import { pendingSubmissionsFor, skillsWithPendingSubmissions, supersededSubmissionIds } from './pending-submissions';

function submission(skillId: string, submissionId: string): ReviewQueueItem {
  return { kind: 'submission', skill_id: skillId, title: skillId, reason: 'a change submitted for review', submission_id: submissionId };
}

const queue: ReviewQueueItem[] = [
  submission('alpha', 'SUB-01M6NAX34J7THX6HNF2M584W8V'),
  { kind: 'draft', skill_id: 'alpha', title: 'alpha', reason: 'new draft' },
  submission('alpha', 'SUB-01M6NB8WMEJCZHFQ8RGPR0PZ2F'),
  submission('beta', 'SUB-01M6DMW0CATJGWRG5C15R2J1D4'),
  submission('alpha', 'SUB-01M6DMW0CATJGWRG5C15R2J1D5'),
];

describe('pending submissions', () => {
  it('lists a skill’s submissions newest first and ignores other kinds and skills', () => {
    expect(pendingSubmissionsFor(queue, 'alpha').map((item) => item.submission_id)).toEqual([
      'SUB-01M6NB8WMEJCZHFQ8RGPR0PZ2F',
      'SUB-01M6NAX34J7THX6HNF2M584W8V',
      'SUB-01M6DMW0CATJGWRG5C15R2J1D5',
    ]);
    expect(pendingSubmissionsFor(queue, 'gamma')).toEqual([]);
    expect(pendingSubmissionsFor(undefined, 'alpha')).toEqual([]);
  });

  it('names the skills that have a waiting submission', () => {
    expect([...skillsWithPendingSubmissions(queue)].sort()).toEqual(['alpha', 'beta']);
  });

  it('marks every submission but the newest of each skill as superseded', () => {
    expect([...supersededSubmissionIds(queue)].sort()).toEqual([
      'SUB-01M6DMW0CATJGWRG5C15R2J1D5',
      'SUB-01M6NAX34J7THX6HNF2M584W8V',
    ]);
  });
});

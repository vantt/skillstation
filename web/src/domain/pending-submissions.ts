import type { ReviewQueueItem } from '../api/types';

// A skill's submissions that wait for review, newest first. Submission ids are ULIDs, so
// they sort by arrival: the first one is the version to review, the rest are older.
export function pendingSubmissionsFor(items: readonly ReviewQueueItem[] | undefined, skillId: string): ReviewQueueItem[] {
  return (items ?? [])
    .filter((item) => item.kind === 'submission' && item.submission_id && item.skill_id === skillId)
    .sort((a, b) => (b.submission_id ?? '').localeCompare(a.submission_id ?? ''));
}

// The ids of skills with at least one waiting submission.
export function skillsWithPendingSubmissions(items: readonly ReviewQueueItem[] | undefined): Set<string> {
  return new Set((items ?? []).filter((item) => item.kind === 'submission' && item.submission_id).map((item) => item.skill_id));
}

// Submission ids that a newer submission of the same skill replaces.
export function supersededSubmissionIds(items: readonly ReviewQueueItem[] | undefined): Set<string> {
  const superseded = new Set<string>();
  for (const skillId of skillsWithPendingSubmissions(items)) {
    for (const older of pendingSubmissionsFor(items, skillId).slice(1)) {
      if (older.submission_id) superseded.add(older.submission_id);
    }
  }
  return superseded;
}

export function submissionReviewPath(submissionId: string): string {
  return `/review/submissions/${encodeURIComponent(submissionId)}`;
}

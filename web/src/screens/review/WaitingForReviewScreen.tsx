import { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router';
import { useReviewQueue } from '../../api/queries';
import type { ReviewQueueItem } from '../../api/types';
import { EmptyState } from '../../components/EmptyState';
import { Skeleton } from '../../components/Skeleton';
import { StatusBadge } from '../../components/StatusBadge';
import { Banner } from '../../components/Banner';
import { pendingSubmissionsFor, submissionReviewPath, supersededSubmissionIds, type ApprovedNotice } from '../../domain/pending-submissions';
import { useT } from '../../i18n';

const TEXT = {
  loadFailed: 'The list of skills waiting for review could not be loaded.',
  title: 'Waiting for review',
  go: 'Review →',
  subtitle: 'Drafts and changed third-party content stay here until a person has looked at them. Approved skills leave the list.',
};


const KIND_LABEL: Record<ReviewQueueItem['kind'], string> = {
  draft: 'New draft',
  content_changed: 'Content changed · needs approval',
  submission: 'Submitted · needs approval',
};

function newestOf(items: readonly ReviewQueueItem[], item: ReviewQueueItem): number {
  return pendingSubmissionsFor(items, item.skill_id).length - 1;
}

function reviewPath(item: ReviewQueueItem): string {
  if (item.submission_id) return submissionReviewPath(item.submission_id);
  return `/skills/${encodeURIComponent(item.skill_id)}/review-content`;
}

// What needs a person before agents get it: new drafts, third-party skills whose content changed
// after it was approved, and skills other machines submitted. Each row opens the full review.
export function WaitingForReviewScreen() {
  const t = useT();
  const { data, isLoading, error } = useReviewQueue();
  const superseded = supersededSubmissionIds(data?.items);
  const location = useLocation();
  const navigate = useNavigate();
  // The notice from an approval lives in the history entry; take it once and clear it so a reload
  // does not show it again.
  const [approved] = useState<ApprovedNotice | null>(() => (location.state as { approved?: ApprovedNotice } | null)?.approved ?? null);
  useEffect(() => {
    if (location.state) navigate(location.pathname, { replace: true, state: null });
  }, [location.state, location.pathname, navigate]);

  if (isLoading) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
        <Skeleton width="40%" height="32px" />
        <Skeleton height="160px" />
      </div>
    );
  }
  if (error || !data) {
    return (
      <div className="fg-banner fg-banner--danger" role="alert">
        <span>{TEXT.loadFailed}</span>
      </div>
    );
  }

  const heading = `${TEXT.title} (${data.items.length})`;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      <header style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
        <h1 className="t-title" style={{ margin: 0, fontSize: '24px', fontWeight: 600 }}>
          {heading}
        </h1>
        <span className="t-body-sm" style={{ color: 'var(--color-text-muted)' }}>
          {TEXT.subtitle}
        </span>
      </header>
      {approved && (
        <Banner
          variant="success"
          role="status"
          actions={
            approved.draft ? <Link to={`/skills/${encodeURIComponent(approved.skillId)}`}>{t('review.open_to_activate')}</Link> : undefined
          }
        >
          {t(approved.draft ? 'review.approved_draft' : 'review.approved_live', { title: approved.title })}
        </Banner>
      )}
      {data.items.length === 0 ? (
        <EmptyState
          title="Nothing is waiting for review"
          description="New drafts and third-party skills whose content changed show up here."
        />
      ) : (
        <ul className="app-review-queue" aria-label="Skills waiting for review">
          {data.items.map((item) => (
            <li
              key={item.submission_id ?? item.skill_id}
              className={`app-review-queue__item app-review-queue__item--${item.kind}`}
            >
              <div className="app-review-queue__main">
                <Link to={reviewPath(item)} className="app-review-queue__title">
                  {item.title || item.skill_id}
                </Link>
                {item.origin && <span className="app-review-queue__origin app-wrap">{item.origin}</span>}
              </div>
              <div className="app-review-queue__meta">
                <StatusBadge label={KIND_LABEL[item.kind]} tone={item.kind === 'draft' ? 'neutral' : 'warning'} />
                {item.submission_id && superseded.has(item.submission_id) && (
                  <StatusBadge label={t('pending.superseded')} tone="neutral" />
                )}
                {item.submission_id && !superseded.has(item.submission_id) && newestOf(data.items, item) > 0 && (
                  <StatusBadge label={t('pending.newest', { count: newestOf(data.items, item) })} tone="success" />
                )}
                {item.since && <span className="t-caption app-review-queue__since">{item.since}</span>}
                {item.submission_id && <span className="t-caption app-review-queue__since">{item.submission_id}</span>}
              </div>
              <span className="app-review-queue__go" aria-hidden="true">
                {TEXT.go}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

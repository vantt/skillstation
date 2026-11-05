import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';
import { ApiError } from '../../api/client';
import { discardSubmission, useReviewQueue } from '../../api/queries';
import { Banner } from '../../components/Banner';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { pendingSubmissionsFor, submissionReviewPath } from '../../domain/pending-submissions';
import { useT } from '../../i18n';

interface PendingSubmissionsNoteProps {
  skillId: string;
}

function errorText(err: unknown): string {
  if (err instanceof ApiError) {
    return [err.render.ERROR, err.render.WHY, err.render.FIX].filter(Boolean).join(' ') || err.message;
  }
  return err instanceof Error ? err.message : String(err);
}

// Says that another machine sent a newer version of this skill which waits for review, and links
// to the newest submission. Older submissions of the same skill can be discarded in one step.
// Shows nothing while the queue loads, when it cannot be read, or when nothing waits.
export function PendingSubmissionsNote({ skillId }: PendingSubmissionsNoteProps) {
  const t = useT();
  const queryClient = useQueryClient();
  const { data } = useReviewQueue();
  const [confirming, setConfirming] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  const pending = pendingSubmissionsFor(data?.items, skillId);
  const [newest, ...older] = pending;
  if (!newest?.submission_id) return null;

  const discardOlder = async () => {
    setDiscarding(true);
    setFailure(null);
    try {
      for (const item of older) {
        if (item.submission_id) await discardSubmission(item.submission_id);
      }
    } catch (err) {
      setFailure(errorText(err));
    } finally {
      setDiscarding(false);
      setConfirming(false);
      queryClient.invalidateQueries({ queryKey: ['review-queue'] });
      queryClient.invalidateQueries({ queryKey: ['home'] });
    }
  };

  return (
    <>
      <Banner
        variant="warning"
        role="status"
        actions={
          <>
            {older.length > 0 && (
              <button type="button" className="fg-btn fg-btn--ghost" disabled={discarding} onClick={() => setConfirming(true)}>
                <span>{discarding ? t('pending.discarding') : t('pending.discard_older')}</span>
              </button>
            )}
            <Link to={submissionReviewPath(newest.submission_id)} className="fg-btn fg-btn--primary">
              <span>{t('pending.review_newest')}</span>
            </Link>
          </>
        }
      >
        <span>
          {t('pending.update_waiting', {
            id: newest.submission_id,
            date: newest.since ?? '',
            origin: (newest.origin ?? '').replace(/^Submitted from /, 'from '),
          })}
        </span>
        {older.length > 0 && (
          <>
            {' '}
            <span>{t('pending.update_waiting_older', { count: older.length })}</span>
          </>
        )}
      </Banner>
      {failure && (
        <Banner variant="danger">
          <span>{failure}</span>
        </Banner>
      )}
      <ConfirmDialog
        open={confirming}
        title={t('pending.discard_older_title')}
        body={t('pending.discard_older_body', { count: older.length, skill: skillId })}
        confirmLabel={discarding ? t('pending.discarding') : t('pending.discard_older')}
        danger
        onConfirm={() => void discardOlder()}
        onCancel={() => setConfirming(false)}
      />
    </>
  );
}

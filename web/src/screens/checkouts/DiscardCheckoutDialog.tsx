import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '../../api/client';
import { confirmCheckoutDiscard, previewCheckoutDiscard } from '../../api/queries';
import type { CheckoutView } from '../../api/types';
import { Skeleton } from '../../components/Skeleton';
import { useT } from '../../i18n';

interface DiscardCheckoutDialogProps {
  checkout: CheckoutView;
  onDiscarded: () => void;
  onCancel: () => void;
}

function failureText(err: unknown): string {
  if (err instanceof ApiError) {
    return [err.render.ERROR, err.render.WHY, err.render.FIX].filter(Boolean).join(' ') || err.message;
  }
  return err instanceof Error ? err.message : String(err);
}

function FileList({ files }: { files: string[] }) {
  return (
    <ul style={{ margin: 0, paddingLeft: 'var(--space-4)', fontFamily: 'var(--font-mono)', fontSize: '13px' }}>
      {files.map((file) => (
        <li key={file}>{file}</li>
      ))}
    </ul>
  );
}

// Asks before a checkout is discarded: it first loads what discarding loses and names it by file,
// then discards the folder exactly as previewed. A folder that changed in between is shown again.
export function DiscardCheckoutDialog({ checkout, onDiscarded, onCancel }: DiscardCheckoutDialogProps) {
  const t = useT();
  const queryClient = useQueryClient();
  const dialogRef = useRef<HTMLDivElement>(null);
  // Loaded fresh each time the dialog opens, and again when the folder changed before the discard.
  const previewQuery = useQuery({
    queryKey: ['checkout-discard-preview', checkout.skill_id, checkout.project_root],
    queryFn: () => previewCheckoutDiscard(checkout.skill_id, checkout.project_root),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const preview = previewQuery.isFetching ? null : (previewQuery.data ?? null);
  const loading = previewQuery.isFetching;
  const [discarding, setDiscarding] = useState(false);
  const [discardError, setDiscardError] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const error = discardError ?? (previewQuery.error && !loading ? failureText(previewQuery.error) : null);

  useEffect(() => {
    dialogRef.current?.focus();
  }, []);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCancel();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onCancel]);

  const handleConfirm = async () => {
    if (!preview) return;
    setDiscarding(true);
    setDiscardError(null);
    setStale(false);
    try {
      await confirmCheckoutDiscard(checkout.skill_id, checkout.project_root, preview.digest);
      onDiscarded();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'stale_proposal') {
        setStale(true);
        await previewQuery.refetch();
      } else {
        setDiscardError(failureText(err));
        // The hub no longer has the record: the list behind the dialog is out of date.
        if (err instanceof ApiError && err.status === 404) {
          void queryClient.invalidateQueries({ queryKey: ['checkouts'] });
        }
      }
    } finally {
      setDiscarding(false);
    }
  };

  const shown = preview?.checkout ?? checkout;
  const gone = shown.state === 'folder_gone';
  const title = gone
    ? t('checkouts.dialog_remove_title', { skill: checkout.skill_id })
    : t('checkouts.dialog_title', { skill: checkout.skill_id });
  const confirmLabel = discarding
    ? t('checkouts.discarding')
    : gone
      ? t('checkouts.confirm_remove')
      : t('checkouts.confirm_discard');

  return (
    <div
      className="fg-scrim"
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
        padding: 'var(--space-4)',
      }}
      onClick={onCancel}
    >
      <div
        ref={dialogRef}
        className="fg-modal"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="discard-checkout-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        style={{
          width: '560px',
          maxWidth: '100%',
          maxHeight: '80vh',
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-4)',
          background: 'var(--color-surface)',
          padding: 'var(--space-4)',
          borderRadius: 'var(--card-radius)',
          boxShadow: 'var(--shadow-lg)',
          outline: 'none',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 'var(--space-3)' }}>
          <div id="discard-checkout-title" className="fg-modal__title" style={{ fontSize: '18px', fontWeight: 600 }}>
            <span>{title}</span>
          </div>
          <button type="button" className="fg-icon-btn" aria-label={t('action.close')} onClick={onCancel}>
            ✕
          </button>
        </div>

        <span style={{ fontSize: '13px', color: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)', overflowWrap: 'anywhere' }}>
          {t('checkouts.dialog_project', { project: checkout.project_root })}
        </span>

        {stale && (
          <div className="fg-banner fg-banner--warning" role="status">
            <span className="fg-banner__dot" />
            <span className="fg-banner__body">
              <span>{t('checkouts.stale')}</span>
            </span>
          </div>
        )}

        {error && (
          <div className="fg-banner fg-banner--danger" role="alert">
            <span className="fg-banner__dot" />
            <span className="fg-banner__body">
              <span>{error}</span>
            </span>
          </div>
        )}

        {loading ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
            <span style={{ fontSize: '13px', color: 'var(--color-text-muted)' }}>{t('checkouts.dialog_loading')}</span>
            <Skeleton height="24px" />
            <Skeleton height="24px" />
          </div>
        ) : preview && gone ? (
          <p style={{ margin: 0, fontSize: '14px' }}>{t('checkouts.folder_gone_body')}</p>
        ) : preview ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)', fontSize: '14px' }}>
            <section style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
              <span style={{ fontWeight: 600 }}>{t('checkouts.unsaved_title')}</span>
              {preview.unsaved.length > 0 ? (
                <FileList files={preview.unsaved} />
              ) : (
                <span style={{ color: 'var(--color-text-muted)' }}>{t('checkouts.unsaved_none')}</span>
              )}
            </section>
            {preview.left_out.length > 0 && (
              <section style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
                <span style={{ fontWeight: 600 }}>{t('checkouts.left_out_title')}</span>
                <span style={{ color: 'var(--color-text-muted)', fontSize: '13px' }}>{t('checkouts.left_out_hint')}</span>
                <FileList files={preview.left_out} />
              </section>
            )}
            <p style={{ margin: 0, color: 'var(--color-text-muted)' }}>
              {shown.new_skill
                ? t('checkouts.hub_keeps_new', { skill: checkout.skill_id })
                : t('checkouts.hub_keeps', { skill: checkout.skill_id })}
            </p>
          </div>
        ) : null}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-2)' }}>
          <button type="button" className="fg-btn fg-btn--ghost" onClick={onCancel}>
            <span>{t('action.cancel')}</span>
          </button>
          <button
            type="button"
            className="fg-btn fg-btn--danger"
            disabled={!preview || loading || discarding}
            onClick={() => void handleConfirm()}
          >
            <span>{confirmLabel}</span>
          </button>
        </div>
      </div>
    </div>
  );
}

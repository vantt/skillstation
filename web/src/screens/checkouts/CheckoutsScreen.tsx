import { useState } from 'react';
import { Link } from 'react-router';
import { useQueryClient } from '@tanstack/react-query';
import { useCheckouts } from '../../api/queries';
import type { CheckoutView } from '../../api/types';
import { Skeleton } from '../../components/Skeleton';
import { StatusBadge } from '../../components/StatusBadge';
import { Toast } from '../../components/Toast';
import { useT } from '../../i18n';
import { checkoutDate } from './checkout-format';
import { DiscardCheckoutDialog } from './DiscardCheckoutDialog';

function checkoutKey(checkout: CheckoutView): string {
  return `${checkout.skill_id}\n${checkout.project_root}`;
}

// Lists the skills checked out into projects on this machine and lets a person discard one.
export function CheckoutsScreen() {
  const t = useT();
  const queryClient = useQueryClient();
  const { data, isLoading, error, refetch } = useCheckouts();
  const [discarding, setDiscarding] = useState<CheckoutView | null>(null);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const handleDiscarded = () => {
    if (discarding) {
      setToastMessage(t('checkouts.discarded', { skill: discarding.skill_id, project: discarding.project_root }));
    }
    setDiscarding(null);
    void queryClient.invalidateQueries({ queryKey: ['checkouts'] });
  };

  if (isLoading) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
        <Skeleton width="30%" height="32px" />
        <Skeleton height="80px" />
        <Skeleton height="80px" />
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
        <div className="fg-banner fg-banner--danger" role="alert">
          <span>{error instanceof Error ? error.message : t('checkouts.load_failed')}</span>
        </div>
        <button type="button" className="fg-btn fg-btn--secondary" style={{ width: 'fit-content' }} onClick={() => void refetch()}>
          <span>{t('action.reload')}</span>
        </button>
      </div>
    );
  }

  const checkouts = data?.checkouts ?? [];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      <h1 style={{ margin: 0, fontSize: '20px', fontWeight: 600 }}>
        <span>{t('checkouts.title')}</span>
      </h1>
      <p style={{ margin: 0, fontSize: '14px', color: 'var(--color-text-muted)', maxWidth: '72ch' }}>
        <span>{t('checkouts.intro')}</span>
      </p>

      {checkouts.length === 0 ? (
        <div
          className="fg-card"
          style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--space-3)', padding: 'var(--space-6) var(--space-4)' }}
        >
          <span style={{ color: 'var(--color-text-muted)', fontSize: '14px', textAlign: 'center' }}>{t('checkouts.empty')}</span>
          <span style={{ color: 'var(--color-text-muted)', fontSize: '13px', textAlign: 'center', maxWidth: '60ch' }}>
            {t('checkouts.empty_hint')}
          </span>
        </div>
      ) : (
        <section className="fg-card" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
          <ul aria-label={t('checkouts.title')} style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
            {checkouts.map((checkout) => {
              const gone = checkout.state === 'folder_gone';
              return (
                <li
                  key={checkoutKey(checkout)}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: 'var(--space-3)',
                    border: '1px solid var(--color-border)',
                    borderRadius: 'var(--radius-sm)',
                    flexWrap: 'wrap',
                    gap: 'var(--space-3)',
                  }}
                >
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)', flex: 1, minWidth: '260px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexWrap: 'wrap' }}>
                      {checkout.new_skill ? (
                        <span style={{ fontWeight: 600, fontSize: '14px' }}>{checkout.skill_id}</span>
                      ) : (
                        <Link
                          to={`/skills/${encodeURIComponent(checkout.skill_id)}`}
                          style={{ color: 'var(--color-link)', textDecoration: 'none', fontWeight: 600, fontSize: '14px' }}
                        >
                          <span>{checkout.skill_id}</span>
                        </Link>
                      )}
                      {checkout.new_skill && <StatusBadge variant="chip" tone="neutral" label={t('checkouts.new_skill')} />}
                    </div>
                    <span style={{ fontSize: '13px', fontFamily: 'var(--font-mono)', color: 'var(--color-text-muted)', overflowWrap: 'anywhere' }}>
                      {checkout.project_root}
                    </span>
                    <div style={{ display: 'flex', gap: 'var(--space-3)', flexWrap: 'wrap', fontSize: '12px', color: 'var(--color-text-muted)' }}>
                      <span>{t('checkouts.started', { date: checkoutDate(checkout.started_at) })}</span>
                      {!gone && <span>{t('checkouts.changed_files', { count: checkout.changed_files })}</span>}
                    </div>
                    {gone ? (
                      <span style={{ fontSize: '13px', color: 'var(--color-warning)' }}>{t('checkouts.folder_gone')}</span>
                    ) : (
                      checkout.official_moved && (
                        <span style={{ fontSize: '13px', color: 'var(--color-warning)' }}>{t('checkouts.official_moved')}</span>
                      )
                    )}
                  </div>
                  <button
                    type="button"
                    className={`fg-btn ${gone ? 'fg-btn--secondary' : 'fg-btn--danger'} fg-btn--small`}
                    aria-label={`${gone ? t('checkouts.remove') : t('checkouts.discard')} ${checkout.skill_id}`}
                    onClick={() => setDiscarding(checkout)}
                  >
                    <span>{gone ? t('checkouts.remove') : t('checkouts.discard')}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        </section>
      )}

      {discarding && (
        <DiscardCheckoutDialog checkout={discarding} onDiscarded={handleDiscarded} onCancel={() => setDiscarding(null)} />
      )}
      <Toast message={toastMessage} onClose={() => setToastMessage(null)} />
    </div>
  );
}

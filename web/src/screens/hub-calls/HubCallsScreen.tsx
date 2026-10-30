import { useHubCalls } from '../../api/queries';
import type { HubCallCount, HubCallEntry } from '../../api/types';
import { Skeleton } from '../../components/Skeleton';
import { StatusBadge } from '../../components/StatusBadge';
import { useT } from '../../i18n';
import { requestLabel, timeOfDay } from './hub-call-format';

const column = { display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' } as const;
const cell = { padding: 'var(--space-2) var(--space-3)', textAlign: 'left', fontSize: '13px' } as const;

function Tally({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div className="fg-card" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)', minWidth: '120px' }}>
      <span style={{ fontSize: '12px', color: 'var(--color-text-muted)' }}>{label}</span>
      <span style={{ fontSize: '24px', fontWeight: 600, color: tone }}>{value}</span>
    </div>
  );
}

function CountList({ title, rows, name }: { title: string; rows: HubCallCount[]; name: (row: HubCallCount) => string }) {
  const t = useT();
  return (
    <section className="fg-card" style={column}>
      <h2 style={{ margin: 0, fontSize: '14px', fontWeight: 600 }}>{title}</h2>
      <ul aria-label={title} style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
        {rows.map((row) => (
          <li key={name(row)} style={{ display: 'flex', justifyContent: 'space-between', gap: 'var(--space-3)', fontSize: '13px' }}>
            <span style={{ fontFamily: 'var(--font-mono)', overflowWrap: 'anywhere' }}>{name(row)}</span>
            <span style={{ color: row.errors > 0 ? 'var(--color-warning)' : 'var(--color-text-muted)' }}>
              {t('hub_calls.calls_errors', { calls: row.calls, errors: row.errors })}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Row({ entry }: { entry: HubCallEntry }) {
  const t = useT();
  const failed = entry.status === 'error';
  return (
    <tr style={{ borderTop: '1px solid var(--color-border)' }}>
      <td style={{ ...cell, whiteSpace: 'nowrap' }}>{timeOfDay(entry.occurred_at)}</td>
      <td style={cell}>{entry.client}</td>
      <td style={{ ...cell, fontFamily: 'var(--font-mono)' }}>{requestLabel(entry)}</td>
      <td style={cell}>
        <StatusBadge variant="chip" tone={failed ? 'danger' : 'success'} label={failed ? t('hub_calls.error') : t('hub_calls.ok')} />
        {failed && entry.error_code && <span style={{ marginLeft: 'var(--space-2)', color: 'var(--color-text-muted)' }}>{entry.error_code}</span>}
      </td>
      <td style={{ ...cell, whiteSpace: 'nowrap' }}>{t('hub_calls.ms', { ms: entry.duration_ms })}</td>
    </tr>
  );
}

// Shows every request clients sent to the hub and its outcome, to check that a client reaches the hub.
export function HubCallsScreen() {
  const t = useT();
  const { data, isLoading, error, refetch } = useHubCalls();

  if (isLoading) {
    return (
      <div style={column}>
        <Skeleton width="30%" height="32px" />
        <Skeleton height="80px" />
        <Skeleton height="160px" />
      </div>
    );
  }
  if (error || !data) {
    return (
      <div style={column}>
        <div className="fg-banner fg-banner--danger" role="alert">
          <span>{error instanceof Error ? error.message : t('hub_calls.load_failed')}</span>
        </div>
        <button type="button" className="fg-btn fg-btn--secondary" style={{ width: 'fit-content' }} onClick={() => void refetch()}>
          <span>{t('action.reload')}</span>
        </button>
      </div>
    );
  }

  return (
    <div style={column}>
      <h1 style={{ margin: 0, fontSize: '20px', fontWeight: 600 }}>
        <span>{t('hub_calls.title')}</span>
      </h1>
      <p style={{ margin: 0, fontSize: '14px', color: 'var(--color-text-muted)', maxWidth: '72ch' }}>
        <span>{t('hub_calls.intro', { days: data.retention_days })}</span>
      </p>

      {data.total === 0 ? (
        <div className="fg-card" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--space-3)', padding: 'var(--space-6) var(--space-4)' }}>
          <span style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>{t('hub_calls.empty', { days: data.retention_days })}</span>
          <span style={{ color: 'var(--color-text-muted)', fontSize: '13px', textAlign: 'center', maxWidth: '60ch' }}>{t('hub_calls.empty_hint')}</span>
        </div>
      ) : (
        <>
          <div style={{ display: 'flex', gap: 'var(--space-3)', flexWrap: 'wrap' }}>
            <Tally label={t('hub_calls.total')} value={data.total} />
            <Tally label={t('hub_calls.succeeded')} value={data.succeeded} />
            <Tally label={t('hub_calls.failed')} value={data.failed} tone={data.failed > 0 ? 'var(--color-warning)' : undefined} />
          </div>
          <div style={{ display: 'grid', gap: 'var(--space-3)', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))' }}>
            <CountList title={t('hub_calls.by_client')} rows={data.by_client} name={(row) => row.client ?? ''} />
            <CountList title={t('hub_calls.by_tool')} rows={data.by_tool} name={(row) => requestLabel(row)} />
            <CountList title={t('hub_calls.by_day')} rows={data.by_day} name={(row) => row.day ?? ''} />
          </div>
          <section className="fg-card" style={{ ...column, overflowX: 'auto' }}>
            <h2 style={{ margin: 0, fontSize: '14px', fontWeight: 600 }}>{t('hub_calls.recent')}</h2>
            <table style={{ borderCollapse: 'collapse', width: '100%' }}>
              <thead>
                <tr style={{ color: 'var(--color-text-muted)' }}>
                  <th style={cell}>{t('hub_calls.col_time')}</th>
                  <th style={cell}>{t('hub_calls.col_client')}</th>
                  <th style={cell}>{t('hub_calls.col_request')}</th>
                  <th style={cell}>{t('hub_calls.col_result')}</th>
                  <th style={cell}>{t('hub_calls.col_duration')}</th>
                </tr>
              </thead>
              <tbody>
                {data.recent.map((entry) => (
                  <Row key={`${entry.occurred_at}-${entry.client}-${entry.method}-${entry.tool ?? ''}`} entry={entry} />
                ))}
              </tbody>
            </table>
          </section>
        </>
      )}
    </div>
  );
}

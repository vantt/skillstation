import { useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';
import { useSkills } from '../../api/queries';
import type { SkillListItem, SkillOrigin, SkillSort } from '../../api/types';
import { CommandBlock } from '../../components/CommandBlock';
import { Skeleton } from '../../components/Skeleton';
import { StatusBadge } from '../../components/StatusBadge';
import { useT } from '../../i18n';

const ICON_SEARCH = '🔍';
const ICON_MORE = '⋯';
const OPT_UPSTREAM_ALL = 'All';
const OPT_UPSTREAM_UPDATES = 'Has an update';
const MSG_NO_UPDATES =
  'No skill has an upstream update right now. Skills added from GitHub are checked for new commits; open one and use Check now, or check them all from a terminal:';
const CMD_CHECK_ALL = 'skillhub skill outdated --check';
const OPT_UPSTREAM_MODIFIED = 'Modified locally';
const OPT_UPSTREAM_UNTRACKED = 'Not tracked';
const SKILL_STATES = ['draft', 'active', 'deprecated', 'archived'] as const;

const ORIGINS: SkillOrigin[] = ['authored', 'learned', 'adopted', 'forked'];
const SORTS: SkillSort[] = ['name', 'added', 'changed', 'uses', 'last-used'];
const ORIGIN_TONE: Record<SkillOrigin, 'neutral' | 'info' | 'success' | 'warning'> = {
  authored: 'neutral',
  learned: 'info',
  adopted: 'success',
  forked: 'warning',
};

// Distinct values, compared without case the way the server filters them.
function distinctValues(lists: (string[] | undefined)[]): string[] {
  const seen = new Map<string, string>();
  for (const list of lists) {
    for (const value of list ?? []) {
      if (!seen.has(value.toLowerCase())) seen.set(value.toLowerCase(), value);
    }
  }
  return Array.from(seen.values()).sort((a, b) => a.localeCompare(b));
}

function oneOf<T extends string>(value: string | null, allowed: readonly T[]): T | undefined {
  return allowed.find((candidate) => candidate === value);
}

export function SkillsScreen() {
  const t = useT();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeMenuId, setActiveMenuId] = useState<string | null>(null);

  const queryParam = searchParams.get('q') ?? '';
  const stateParam = searchParams.get('state') ?? 'all';
  const originParam = searchParams.get('origin') ?? 'all';
  const topicParam = searchParams.get('topic') ?? 'all';
  const labelParam = searchParams.get('label') ?? 'all';
  const sortParam = oneOf(searchParams.get('sort'), SORTS) ?? 'name';
  const colParam = searchParams.get('collection') ?? 'all';
  const upstreamParam = searchParams.get('upstream') ?? 'all';
  const onlyUpdatesFilter =
    upstreamParam === 'updates' &&
    !queryParam &&
    [stateParam, originParam, topicParam, labelParam, colParam].every((value) => value === 'all');

  // Filters, search and order run on the server; collection and upstream stay client-side
  // because the listing query does not cover them.
  const { data, isLoading, error } = useSkills({
    state: oneOf(stateParam, SKILL_STATES),
    origin: oneOf(originParam, ORIGINS),
    topic: topicParam === 'all' ? undefined : topicParam,
    label: labelParam === 'all' ? undefined : labelParam,
    q: queryParam || undefined,
    sort: sortParam === 'name' ? undefined : sortParam,
  });
  // The unfiltered list supplies the filter options and tells "no skills" from "no match".
  const { data: everything } = useSkills();

  const allSkills = everything?.skills ?? [];
  const updateCount = allSkills.filter(
    (s) => s.upstream_status === 'update_available' || s.upstream_status === 'diverged',
  ).length;
  const updatesBadgeText = `Updates (${updateCount})`;

  const collections = Array.from(new Set(allSkills.map((s) => s.collection).filter(Boolean))).sort();
  const topics = distinctValues(allSkills.flatMap((s) => [s.topics, s.technologies]));
  const labels = distinctValues(allSkills.map((s) => s.labels));

  const filteredSkills = (data?.skills ?? []).filter((s) => {
    if (colParam !== 'all' && s.collection !== colParam) {
      return false;
    }
    if (upstreamParam === 'updates') {
      return s.upstream_status === 'update_available' || s.upstream_status === 'diverged';
    }
    if (upstreamParam === 'modified') {
      return s.upstream_status === 'modified' || s.upstream_status === 'diverged';
    }
    if (upstreamParam === 'untracked') {
      return s.upstream_status === 'untracked';
    }
    return true;
  });

  const usageAvailable = data?.usage_available ?? false;
  const showNoUsageNote = Boolean(data) && !usageAvailable && (sortParam === 'uses' || sortParam === 'last-used');
  const noSkillsAtAll = everything !== undefined && allSkills.length === 0;

  // Every control writes its value to the URL; the default value removes the parameter.
  const setParam = (key: string, value: string, defaultValue = 'all') => {
    const next = new URLSearchParams(searchParams);
    if (value && value !== defaultValue) {
      next.set(key, value);
    } else {
      next.delete(key);
    }
    setSearchParams(next);
  };

  const clearFilters = () => {
    setSearchParams(new URLSearchParams());
  };

  const isFallback = Boolean(data?.summary && data.summary.toLowerCase().includes('fallback'));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      {/* Controls Bar */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 'var(--space-3)',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-3)', alignItems: 'flex-end', flex: 1 }}>
          <div className="app-field app-search" style={{ flex: '1 1 220px', maxWidth: '340px' }}>
            <label htmlFor="skills-search">{t('skills.search_label')}</label>
            <div className="fg-search">
              <span className="fg-search__glyph" aria-hidden="true">
                {ICON_SEARCH}
              </span>
              <input
                id="skills-search"
                type="search"
                className="fg-input"
                placeholder={t('skills.search_placeholder')}
                value={queryParam}
                onChange={(e) => setParam('q', e.target.value, '')}
              />
            </div>
          </div>

          <div className="app-field" style={{ width: '160px' }}>
            <label htmlFor="skills-filter-state">{t('skills.filter_lifecycle')}</label>
            <div className="fg-select">
            <select
              id="skills-filter-state"
              value={stateParam}
              onChange={(e) => setParam('state', e.target.value)}
            >
              <option value="all">{t('lifecycle.all')}</option>
              <option value="draft">{t('lifecycle_short.draft')}</option>
              <option value="active">{t('lifecycle_short.active')}</option>
              <option value="deprecated">{t('lifecycle_short.deprecated')}</option>
              <option value="archived">{t('lifecycle_short.archived')}</option>
            </select>
            <span className="fg-select__chev" aria-hidden="true">
              ▾
            </span>
            </div>
          </div>

          <FilterSelect
            id="skills-filter-origin"
            label={t('skills.filter_origin')}
            value={originParam}
            onChange={(value) => setParam('origin', value)}
            options={[
              { value: 'all', label: t('lifecycle.all') },
              ...ORIGINS.map((origin) => ({ value: origin, label: t(`origin.${origin}`) })),
            ]}
          />

          <FilterSelect
            id="skills-filter-topic"
            label={t('skills.filter_topic')}
            value={topicParam}
            onChange={(value) => setParam('topic', value)}
            options={[{ value: 'all', label: t('lifecycle.all') }, ...withCurrent(topics, topicParam)]}
          />

          <FilterSelect
            id="skills-filter-label"
            label={t('skills.filter_label')}
            value={labelParam}
            onChange={(value) => setParam('label', value)}
            options={[{ value: 'all', label: t('lifecycle.all') }, ...withCurrent(labels, labelParam)]}
          />

          <div className="app-field" style={{ width: '170px' }}>
            <label htmlFor="skills-filter-collection">{t('skills.filter_collection')}</label>
            <div className="fg-select">
            <select
              id="skills-filter-collection"
              value={colParam}
              onChange={(e) => setParam('collection', e.target.value)}
            >
              <option value="all">{t('lifecycle.all')}</option>
              {collections.map((col) => (
                <option key={col} value={col}>
                  {col}
                </option>
              ))}
            </select>
            <span className="fg-select__chev" aria-hidden="true">
              ▾
            </span>
            </div>
          </div>

          <div className="app-field" style={{ width: '160px' }}>
            <label htmlFor="skills-filter-upstream">{t('skills.filter_upstream')}</label>
            <div className="fg-select">
            <select
              id="skills-filter-upstream"
              value={upstreamParam}
              onChange={(e) => setParam('upstream', e.target.value)}
            >
              <option value="all">{OPT_UPSTREAM_ALL}</option>
              <option value="updates">{OPT_UPSTREAM_UPDATES}</option>
              <option value="modified">{OPT_UPSTREAM_MODIFIED}</option>
              <option value="untracked">{OPT_UPSTREAM_UNTRACKED}</option>
            </select>
            <span className="fg-select__chev" aria-hidden="true">
              ▾
            </span>
            </div>
          </div>

          <FilterSelect
            id="skills-sort"
            label={t('skills.sort_label')}
            value={sortParam}
            onChange={(value) => setParam('sort', value, 'name')}
            options={SORTS.map((sort) => ({ value: sort, label: t(`skills.sort.${sort}`) }))}
          />

          {updateCount > 0 && (
            <button
              type="button"
              className="fg-chip fg-chip--warning"
              style={{ cursor: 'pointer', border: 'none' }}
              onClick={() => setParam('upstream', 'updates')}
            >
              <span>{updatesBadgeText}</span>
            </button>
          )}
        </div>

        <div className="app-actions" style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-2)' }}>
          <Link to="/skills/add" className="fg-btn fg-btn--secondary" style={{ textDecoration: 'none' }}>
            <span>{t('action.add_from_github')}</span>
          </Link>
          <Link to="/skills/create" className="fg-btn fg-btn--primary" style={{ textDecoration: 'none' }}>
            <span>{t('action.create_skill')}</span>
          </Link>
        </div>
      </div>

      {/* Fallback Banner */}
      {isFallback && data?.summary && (
        <div className="fg-banner fg-banner--info" role="status">
          <span className="fg-banner__dot" />
          <span className="fg-banner__body">
            <span>{data.summary}</span>
          </span>
        </div>
      )}

      {showNoUsageNote && (
        <div className="fg-banner fg-banner--info" role="status">
          <span className="fg-banner__dot" />
          <span className="fg-banner__body">
            <span>{t('skills.usage_unavailable_note')}</span>
          </span>
        </div>
      )}

      {/* Loading Skeleton */}
      {isLoading && (
        <div className="fg-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <Skeleton />
          <Skeleton />
          <Skeleton />
          <Skeleton />
        </div>
      )}

      {/* Error View */}
      {error && !isLoading && (
        <div className="fg-card fg-card--danger">
          <span className="t-body">
            <span>{t('common.error')}</span>
          </span>
        </div>
      )}

      {/* Empty State - No skills in workspace */}
      {!isLoading && !error && noSkillsAtAll && (
        <div className="fg-card">
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              padding: 'var(--space-8) var(--space-4)',
              textAlign: 'center',
              gap: 'var(--space-3)',
            }}
          >
            <span className="t-body">
              <span>{t('skills.empty_none')}</span>
            </span>
            <Link to="/skills/add" className="fg-btn fg-btn--primary" style={{ textDecoration: 'none' }}>
              <span>{t('action.add_from_github')}</span>
            </Link>
          </div>
        </div>
      )}

      {/* Filtered Empty State */}
      {!isLoading && !error && data && !noSkillsAtAll && filteredSkills.length === 0 && (
        <div className="fg-card">
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              padding: 'var(--space-8) var(--space-4)',
              textAlign: 'center',
              gap: 'var(--space-3)',
            }}
          >
            <span className="t-body">
              <span>{onlyUpdatesFilter ? MSG_NO_UPDATES : t('skills.empty_filtered')}</span>
            </span>
            {onlyUpdatesFilter && <CommandBlock command={CMD_CHECK_ALL} />}
            <button type="button" className="fg-btn fg-btn--secondary" onClick={clearFilters}>
              <span>{t('action.clear_filters')}</span>
            </button>
          </div>
        </div>
      )}

      {/* Table / Card List */}
      {!isLoading && !error && filteredSkills.length > 0 && (
        <div className="fg-card" style={{ padding: 0, overflow: 'visible' }}>
          <table className="fg-table fg-table--clickable app-skills" style={{ width: '100%' }}>
            <thead>
              <tr>
                <th>
                  <span>{t('skills.col_skill')}</span>
                </th>
                <th>
                  <span>{t('skills.col_origin')}</span>
                </th>
                <th>
                  <span>{t('skills.col_collection')}</span>
                </th>
                <th>
                  <span>{t('skills.col_lifecycle')}</span>
                </th>
                <th>
                  <span>{t('skills.col_routing')}</span>
                </th>
                <th>
                  <span>{t('skills.col_usage')}</span>
                </th>
                <th style={{ width: '48px' }}>
                  <span style={{ position: 'absolute', left: '-9999px' }}>
                    <span>{t('skills.col_actions')}</span>
                  </span>
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredSkills.map((s: SkillListItem) => {
                const isMenuOpen = activeMenuId === s.id;
                const canDeprecate = s.lifecycle_state === 'active';
                const canArchive = s.lifecycle_state === 'deprecated';

                const tone =
                  s.lifecycle_state === 'active'
                    ? 'success'
                    : s.lifecycle_state === 'draft'
                      ? 'warning'
                      : s.lifecycle_state === 'deprecated'
                        ? 'danger'
                        : 'neutral';

                return (
                  <tr
                    key={s.id}
                    onClick={() => navigate(`/skills/${s.id}`)}
                    style={{ cursor: 'pointer' }}
                  >
                    <td>
                      <div className="t-ui">
                        <span>{s.name}</span>
                      </div>
                      <div style={{ fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--color-text-muted)' }}>
                        <span>{s.id}</span>
                      </div>
                      {s.labels && s.labels.length > 0 && (
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-1)', marginTop: 'var(--space-1)' }}>
                          {s.labels.map((label) => (
                            <StatusBadge key={label} variant="chip" tone="info" label={label} />
                          ))}
                        </div>
                      )}
                    </td>
                    <td data-label={t('skills.col_origin')}>
                      <span title={t(`origin.${s.origin}_hint`)}>
                        <StatusBadge variant="chip" tone={ORIGIN_TONE[s.origin] ?? 'neutral'} label={t(`origin.${s.origin}`)} />
                      </span>
                    </td>
                    <td className="t-body-sm" data-label={t('skills.col_collection')}>
                      <span>{s.collection}</span>
                    </td>
                    <td data-label={t('skills.col_lifecycle')}>
                      <div style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--space-1)', flexWrap: 'wrap' }}>
                        <StatusBadge
                          variant="chip"
                          label={t(`lifecycle_short.${s.lifecycle_state}`)}
                          tone={tone}
                        />
                        {s.upstream_status === 'update_available' && (
                          <StatusBadge variant="chip" tone="warning" label="Update available" />
                        )}
                        {s.upstream_status === 'diverged' && (
                          <StatusBadge variant="chip" tone="danger" label="Diverged" />
                        )}
                        {s.upstream_status === 'upstream_removed' && (
                          <StatusBadge variant="chip" tone="danger" label="Removed upstream" />
                        )}
                      </div>
                    </td>
                    <td className="t-body-sm" data-label={t('skills.col_routing')}>
                      <span style={{ color: s.routing_eligible ? 'var(--color-text)' : 'var(--color-text-muted)' }}>
                        <span>{s.routing_eligible ? t('routing.routable') : t('routing.not_routed')}</span>
                      </span>
                    </td>
                    <td className="t-body-sm" data-label={t('skills.col_usage')}>
                      <UsageText skill={s} available={usageAvailable} />
                    </td>
                    <td
                      className="app-skills__actions"
                      onClick={(e) => {
                        e.stopPropagation();
                      }}
                    >
                      <button
                        type="button"
                        className="fg-icon-btn"
                        aria-label={`Actions for ${s.name}`}
                        aria-haspopup="menu"
                        aria-expanded={isMenuOpen}
                        onClick={() => setActiveMenuId(isMenuOpen ? null : s.id)}
                      >
                        {ICON_MORE}
                      </button>

                      {isMenuOpen && (
                        <div
                          className="fg-menu"
                          role="menu"
                          style={{
                            position: 'absolute',
                            right: '8px',
                            top: 'calc(100% - 4px)',
                            zIndex: 40,
                            display: 'flex',
                            flexDirection: 'column',
                            minWidth: '120px',
                            background: 'var(--color-surface-raised)',
                            border: '1px solid var(--color-border)',
                            borderRadius: 'var(--card-radius)',
                            boxShadow: 'var(--shadow-sm)',
                          }}
                        >
                          <Link
                            to={`/skills/${s.id}`}
                            className="fg-menu__item"
                            style={{ textDecoration: 'none', padding: '8px 12px', color: 'var(--color-text)' }}
                            onClick={() => setActiveMenuId(null)}
                          >
                            <span>{t('action.review')}</span>
                          </Link>

                          {canDeprecate && (
                            <Link
                              to={`/skills/${s.id}?tab=review`}
                              className="fg-menu__item"
                              style={{ textDecoration: 'none', padding: '8px 12px', color: 'var(--color-text)' }}
                              onClick={() => setActiveMenuId(null)}
                            >
                              <span>{t('action.deprecate')}</span>
                            </Link>
                          )}

                          {canArchive && (
                            <Link
                              to={`/skills/${s.id}?tab=review`}
                              className="fg-menu__item fg-menu__item--danger"
                              style={{ textDecoration: 'none', padding: '8px 12px', color: 'var(--color-danger)' }}
                              onClick={() => setActiveMenuId(null)}
                            >
                              <span>{t('action.archive')}</span>
                            </Link>
                          )}
                        </div>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// A selected value stays an option even when the unfiltered list no longer has it, so the
// control never shows a value that is not in its menu.
function withCurrent(values: string[], current: string): { value: string; label: string }[] {
  const all = current !== 'all' && !values.some((v) => v.toLowerCase() === current.toLowerCase()) ? [...values, current] : values;
  return all.map((value) => ({ value, label: value }));
}

function FilterSelect({
  id,
  label,
  value,
  options,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}) {
  return (
    <div className="app-field" style={{ width: '160px' }}>
      <label htmlFor={id}>{label}</label>
      <div className="fg-select">
        <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <span className="fg-select__chev" aria-hidden="true">
          ▾
        </span>
      </div>
    </div>
  );
}

// Without any recorded load a zero would read as "never used", so the cell says there is no data.
function UsageText({ skill, available }: { skill: SkillListItem; available: boolean }) {
  const t = useT();
  if (!available) {
    return <span style={{ color: 'var(--color-text-muted)' }}>{t('skills.usage_unknown')}</span>;
  }
  const title = skill.last_used ? t('skills.last_used', { date: skill.last_used.slice(0, 10) }) : t('skills.never_used');
  return (
    <span title={title} style={{ color: skill.uses_30d > 0 ? 'var(--color-text)' : 'var(--color-text-muted)' }}>
      {t('skills.uses', { count: skill.uses_30d })}
    </span>
  );
}

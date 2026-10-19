import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SkillsScreen } from './SkillsScreen';
import { loadGolden } from '../../test/golden';
import type { SkillListItem, SkillListResult } from '../../api/types';

function row(overrides: Partial<SkillListItem> & Pick<SkillListItem, 'id' | 'name'>): SkillListItem {
  return {
    collection: 'core',
    lifecycle_state: 'active',
    state: 'active',
    active_locally: true,
    routing_eligible: true,
    origin: 'authored',
    uses_30d: 0,
    ...overrides,
  };
}

function list(skills: SkillListItem[], usageAvailable = true): SkillListResult {
  return { schema_version: '1', status: 'ok', summary: `${skills.length} skills`, skills, usage_available: usageAvailable };
}

// The stubbed server answers the unfiltered list with `data` and any filtered request with
// `filtered`, recording every URL it was asked for. The unfiltered list is also cached up front
// so the first render is synchronous.
function renderSkillsScreen(data: SkillListResult, initialEntries = ['/skills'], filtered: SkillListResult = data) {
  const requested: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      requested.push(url);
      return new Response(JSON.stringify(url.includes('?') ? filtered : data), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }),
  );
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(['skills', ''], data);

  const view = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <Routes>
          <Route path="/skills" element={<SkillsScreen />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...view, requested };
}

describe('SkillsScreen', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders with golden skills.json', () => {
    const golden = loadGolden<SkillListResult>('skills');
    renderSkillsScreen(golden);

    expect(screen.getByText('Review Skill')).toBeInTheDocument();
    expect(screen.getByText('review-skill')).toBeInTheDocument();
    expect(screen.getAllByText('core').length).toBeGreaterThan(0);
    expect(screen.getByRole('cell', { name: 'Authored' })).toBeInTheDocument();
  });

  it('renders update_available chip and filters by ?upstream=updates', () => {
    const data = list([
      row({ id: 'pdf', name: 'PDF Tool', upstream_status: 'update_available' }),
      row({ id: 'docx', name: 'Docx Tool', upstream_status: 'up_to_date' }),
    ]);

    // Render with no filter
    const { unmount } = renderSkillsScreen(data);
    expect(screen.getByText('Update available')).toBeInTheDocument();
    expect(screen.getByText('Updates (1)')).toBeInTheDocument();
    expect(screen.getByText('PDF Tool')).toBeInTheDocument();
    expect(screen.getByText('Docx Tool')).toBeInTheDocument();
    unmount();

    // Render with ?upstream=updates
    renderSkillsScreen(data, ['/skills?upstream=updates']);
    expect(screen.getByText('PDF Tool')).toBeInTheDocument();
    expect(screen.queryByText('Docx Tool')).not.toBeInTheDocument();
  });

  it('explains an empty upstream-updates list and offers the way to check', () => {
    renderSkillsScreen(list([row({ id: 'docx', name: 'Docx Tool', upstream_status: 'up_to_date' })]), [
      '/skills?upstream=updates',
    ]);
    expect(screen.getByText(/No skill has an upstream update right now/)).toBeInTheDocument();
    expect(screen.getByText('skillstation skill outdated --check')).toBeInTheDocument();
    // The filter is a visible control, so no URL has to be typed.
    expect(screen.getByLabelText('Upstream')).toHaveValue('updates');
    expect(screen.getByRole('option', { name: 'Has an update' })).toBeInTheDocument();
  });

  it('asks the server for the chosen filters and sort and shows origin, labels and the no-usage note', async () => {
    const everything = list(
      [
        row({ id: 'code-review', name: 'Code Review', topics: ['review'], labels: ['daily'] }),
        row({ id: 'pdf', name: 'PDF Tool', origin: 'adopted', technologies: ['go'], labels: ['docs'] }),
      ],
      false,
    );
    const answer = list([row({ id: 'pdf', name: 'PDF Tool', origin: 'adopted', technologies: ['go'], labels: ['docs'] })], false);
    const { requested } = renderSkillsScreen(everything, ['/skills?q=pdf&topic=go&label=docs&sort=uses'], answer);

    // Topic and label options come from the unfiltered list.
    expect(screen.getByRole('option', { name: 'review' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'daily' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Origin'), { target: { value: 'adopted' } });

    await waitFor(() =>
      expect(requested).toContain('/api/v1/skills?origin=adopted&topic=go&label=docs&q=pdf&sort=uses'),
    );
    expect(await screen.findByText(/Usage is not available yet/)).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('Code Review')).not.toBeInTheDocument());
    expect(screen.getAllByText('Adopted').length).toBeGreaterThan(0);
    expect(screen.getAllByText('docs').length).toBeGreaterThan(0);
    // Without recorded loads a row says so instead of showing zero uses.
    expect(screen.getByText('No usage data yet')).toBeInTheDocument();
    expect(screen.queryByText('0 uses (30d)')).toBeNull();
  });

  it('shows 30-day uses with the last-used date', () => {
    renderSkillsScreen(list([row({ id: 'pdf', name: 'PDF Tool', uses_30d: 3, last_used: '2026-10-16' })]));
    expect(screen.getByText('3 uses (30d)')).toHaveAttribute('title', 'Last used 2026-10-16');
    expect(screen.queryByText(/Usage is not available yet/)).toBeNull();
  });

  it('empty list shows "No skills yet."', () => {
    renderSkillsScreen(list([]));
    expect(screen.getByText('No skills yet.')).toBeInTheDocument();
  });

  it('filtered-empty shows "No skills match these filters." with Clear filters', async () => {
    renderSkillsScreen(list([row({ id: 'test-skill', name: 'Test Skill' })]), ['/skills?q=nomatch'], list([]));

    expect(await screen.findByText('No skills match these filters.')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Clear filters'));
    expect(await screen.findByText('Test Skill')).toBeInTheDocument();
  });

  it('renders row menu items per lifecycle state', () => {
    renderSkillsScreen(
      list([
        row({ id: 'active-skill', name: 'Active Skill' }),
        row({ id: 'deprecated-skill', name: 'Deprecated Skill', lifecycle_state: 'deprecated', state: 'deprecated', active_locally: false, routing_eligible: false }),
        row({ id: 'draft-skill', name: 'Draft Skill', lifecycle_state: 'draft', state: 'draft', active_locally: false, routing_eligible: false }),
      ]),
    );

    // 1. Active skill menu has Review and Deprecate
    fireEvent.click(screen.getByLabelText('Actions for Active Skill'));
    expect(screen.getByText('Review')).toBeInTheDocument();
    expect(screen.getByText('Deprecate')).toBeInTheDocument();
    expect(screen.queryByText('Archive')).toBeNull();

    // 2. Deprecated skill menu has Review and Archive
    fireEvent.click(screen.getByLabelText('Actions for Deprecated Skill'));
    expect(screen.getByText('Archive')).toBeInTheDocument();
    expect(screen.queryByText('Deprecate')).toBeNull();

    // 3. Draft skill menu has Review only
    fireEvent.click(screen.getByLabelText('Actions for Draft Skill'));
    expect(screen.getByText('Review')).toBeInTheDocument();
    expect(screen.queryByText('Deprecate')).toBeNull();
    expect(screen.queryByText('Archive')).toBeNull();
  });

  it('labels every filter', () => {
    renderSkillsScreen(list([row({ id: 'code-review', name: 'Quality Gate', collection: 'engineering' })]));

    for (const label of ['Search', 'Lifecycle', 'Origin', 'Topic', 'Label', 'Collection', 'Upstream', 'Sort by']) {
      expect(screen.getByLabelText(label)).toBeInTheDocument();
    }
  });
});

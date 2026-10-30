import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import type { HubCallLog } from '../../api/types';

const log: HubCallLog = {
  retention_days: 3,
  capture_task_text: true,
  since: '2026-10-27T10:00:00Z',
  total: 3,
  succeeded: 2,
  failed: 1,
  by_tool: [{ method: 'tools_call', tool: 'skill_get', calls: 1, errors: 1 }, { method: 'connect', calls: 1, errors: 0 }],
  by_day: [{ day: '2026-10-30', calls: 2, errors: 1 }],
  by_client: [{ client: 'claude-code', calls: 2, errors: 1 }],
  recent: [
    { occurred_at: '2026-10-30T10:00:00Z', method: 'tools_call', tool: 'skill_get', status: 'error', error_code: 'not_found', duration_ms: 3, client: 'claude-code' },
    {
      occurred_at: '2026-10-30T09:59:30Z', method: 'tools_call', tool: 'skill_resolve', status: 'ok', duration_ms: 20, client: 'claude-code',
      operation: 'review', task_text: 'review my pull request',
      resolution: { id: 'res_1', status: 'resolved', recommended: ['code-review'], loaded: ['code-review'], used: false },
    },
    { occurred_at: '2026-10-30T09:59:00Z', method: 'connect', status: 'ok', duration_ms: 1, client: 'claude-code' },
  ],
};

vi.mock('../../api/queries', async (orig) => {
  const actual = await orig<typeof import('../../api/queries')>();
  return { ...actual, useHubCalls: () => ({ data: log, isLoading: false, error: null, refetch: vi.fn() }) };
});

import { HubCallsScreen } from './HubCallsScreen';

describe('HubCallsScreen', () => {
  it('shows the totals and each call with its outcome', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <HubCallsScreen />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getByText('Succeeded')).toBeTruthy();
    const latest = within(screen.getByRole('table'));
    expect(latest.getByText('skill_get')).toBeTruthy();
    expect(latest.getByText('not_found')).toBeTruthy();
    expect(latest.getByText('connect')).toBeTruthy();
    expect(screen.getByLabelText('By client').textContent).toContain('claude-code');
    // A resolve call shows its task text, the skill it recommended and what followed.
    expect(latest.getByText('review my pull request')).toBeTruthy();
    expect(latest.getByText('code-review')).toBeTruthy();
    expect(latest.getByText('loaded · not used')).toBeTruthy();
    expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(true);
  });
});

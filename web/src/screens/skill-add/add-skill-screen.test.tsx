import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SkillAddProposal } from '../../api/types';
import { ApiError } from '../../api/client';

const previewSkillAdd = vi.fn();
const previewSkillZip = vi.fn();

vi.mock('../../api/queries', async (orig) => {
  const actual = await orig<typeof import('../../api/queries')>();
  return {
    ...actual,
    previewSkillAdd: (...args: unknown[]) => previewSkillAdd(...args),
    previewSkillZip: (...args: unknown[]) => previewSkillZip(...args),
  };
});

import { AddSkillScreen } from './AddSkillScreen';

function proposal(over: Partial<SkillAddProposal> = {}): SkillAddProposal {
  return {
    schema_version: '1',
    status: 'ok',
    summary: 'Add demo-skill',
    skill_id: 'demo-skill',
    collection: 'default',
    diff: { added: ['skills/default/demo-skill/SKILL.md'], modified: [], deleted: [] },
    confirmation: {
      confirmation: { pins: { proposal_id: 'PROP-1', proposal_digest: 'sha256:d', base_version: 'v1' } },
    },
    ...over,
  };
}

async function previewWith(result: SkillAddProposal) {
  previewSkillAdd.mockResolvedValue(result);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <AddSkillScreen />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  fireEvent.change(document.getElementById('gh-url') as HTMLInputElement, {
    target: { value: 'https://github.com/acme/skills/tree/main/demo-skill' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Discover skills' }));
  await screen.findByText('Files that would be written');
}

describe('AddSkillScreen review step', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists the files that were left out, with the reason', async () => {
    await previewWith(proposal({ excluded: [{ path: '.env', reason: 'may hold secrets' }] }));

    const leftOut = screen.getByText('Left out (not copied into the hub)').closest('section') as HTMLElement;
    expect(within(leftOut).getByText('.env')).toBeInTheDocument();
    expect(within(leftOut).getByText(/may hold secrets/)).toBeInTheDocument();
  });

  it('shows no left-out list when nothing was left out', async () => {
    await previewWith(proposal());

    expect(screen.queryByText('Left out (not copied into the hub)')).not.toBeInTheDocument();
  });
});

describe('AddSkillScreen collection', () => {
  it('files imported skills under the default collection unless told otherwise', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <AddSkillScreen />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Advanced'));
    expect((document.getElementById('gh-col') as HTMLInputElement).value).toBe('default');
    expect(screen.getByText(/Pick default if unsure/)).toBeInTheDocument();
    expect(screen.queryByText(/Leave it as core/)).not.toBeInTheDocument();
  });
});

function renderAt(entry: string) {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[entry]}>
        <AddSkillScreen />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('AddSkillScreen source', () => {
  it('opens on the zip upload when the link asks for it, with only the file picker', () => {
    renderAt('/skills/add?source=zip');

    expect(screen.getByRole('button', { name: 'Upload zip' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText(/Upload a \.zip file/)).toBeInTheDocument();
    expect(screen.getByText(/up to 10 MiB\. Each skill needs a SKILL\.md/)).toBeInTheDocument();
    expect(document.getElementById('zip-file')).not.toBeNull();
    expect(document.getElementById('gh-url')).toBeNull();
    expect(screen.getByRole('button', { name: 'Upload and discover skills' })).toBeDisabled();
  });

  it('opens on the GitHub URL for a missing or unknown source', () => {
    renderAt('/skills/add?source=ftp');

    expect(screen.getByRole('button', { name: 'Add from GitHub' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText(/Paste a GitHub URL/)).toBeInTheDocument();
    expect(document.getElementById('gh-url')).not.toBeNull();
    expect(document.getElementById('zip-file')).toBeNull();
  });

  it('drops the chosen file and the selection prompt when switching source', async () => {
    previewSkillAdd.mockRejectedValue(
      new ApiError(400, {
        code: 'skill_selection_required',
        render: { ERROR: 'Pick a skill.', WHY: 'The repository holds several skills.', FIX: 'Name one.' },
      }),
    );
    renderAt('/skills/add?source=github');
    fireEvent.change(document.getElementById('gh-url') as HTMLInputElement, {
      target: { value: 'https://github.com/acme/skills' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Discover skills' }));
    await screen.findByText('The repository holds several skills.');

    fireEvent.click(screen.getByRole('button', { name: 'Upload zip' }));
    expect(screen.queryByText('The repository holds several skills.')).not.toBeInTheDocument();
    expect(screen.queryByText('Skill name or path')).not.toBeInTheDocument();

    const archive = new File(['zip'], 'demo-skill.zip', { type: 'application/zip' });
    fireEvent.change(document.getElementById('zip-file') as HTMLInputElement, { target: { files: [archive] } });
    expect(screen.getByRole('button', { name: 'Upload and discover skills' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Add from GitHub' }));
    expect(document.getElementById('zip-file')).toBeNull();
    // The typed URL is visible again, so keeping it hides nothing.
    expect((document.getElementById('gh-url') as HTMLInputElement).value).toBe('https://github.com/acme/skills');

    fireEvent.click(screen.getByRole('button', { name: 'Upload zip' }));
    expect(screen.getByRole('button', { name: 'Upload and discover skills' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Upload and discover skills' }));
    expect(previewSkillZip).not.toHaveBeenCalled();
  });
});

describe('AddSkillScreen zip upload', () => {
  function renderScreen() {
    renderAt('/skills/add?source=zip');
  }

  it('sends the chosen archive and shows the same review step', async () => {
    previewSkillZip.mockResolvedValue(proposal({ origin: { kind: 'archive' } } as Partial<SkillAddProposal>));
    renderScreen();
    const archive = new File(['zip'], 'demo-skill.zip', { type: 'application/zip' });
    fireEvent.change(document.getElementById('zip-file') as HTMLInputElement, { target: { files: [archive] } });
    fireEvent.click(screen.getByRole('button', { name: 'Upload and discover skills' }));

    await screen.findByText('Files that would be written');
    expect(previewSkillZip).toHaveBeenCalledWith(archive, expect.objectContaining({ collection: 'default', all: false }), expect.anything());
    expect(screen.getByText(/agents receive none of it until you read every file and approve it/)).toBeInTheDocument();
  });

  it('refuses a file that is not a zip before uploading it', () => {
    renderScreen();
    const notes = new File(['x'], 'notes.txt', { type: 'text/plain' });
    fireEvent.change(document.getElementById('zip-file') as HTMLInputElement, { target: { files: [notes] } });
    fireEvent.click(screen.getByRole('button', { name: 'Upload and discover skills' }));

    expect(screen.getByText('Choose a file that ends in .zip.')).toBeInTheDocument();
    expect(previewSkillZip).not.toHaveBeenCalled();
  });
});

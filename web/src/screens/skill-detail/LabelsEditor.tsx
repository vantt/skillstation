import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ApiError } from '../../api/client';
import { commitNote } from '../../api/commit-view';
import { proposalPatch, proposalPaths, proposalWarning } from '../../api/proposal-view';
import { confirmSkillMutation, previewSkillUpdate } from '../../api/queries';
import type { SkillDetail, SkillProposal } from '../../api/types';
import { ProposalPreview } from '../../components/ProposalPreview';
import { Toast } from '../../components/Toast';
import { splitList } from './edit-fields';

const LABEL_LABELS = 'Labels';
const LABEL_ADD = 'Add';
const LABEL_RESET = 'Reset';
const LABEL_PREVIEW = 'Preview label changes';
const LABEL_VALIDATING = 'Validating…';
const LABEL_NONE = 'No labels yet.';
const LABEL_CURRENT = 'Current labels';
const PLACEHOLDER_ADD = 'Add a label, e.g. favourite';
const HINT_LABELS = 'Your own grouping for the Skills list. Labels never change routing.';
// The metadata schema caps each label at 64 characters.
const MAX_LABEL_LENGTH = 64;

// Adds labels typed as one or more comma-separated values, skipping ones already present in any case.
function addLabels(current: string[], typed: string): string[] {
  const next = [...current];
  for (const label of splitList(typed)) {
    if (!next.some((existing) => existing.toLowerCase() === label.toLowerCase())) next.push(label);
  }
  return next;
}

function describeLabelChange(before: string[], after: string[]): string {
  const added = after.filter((x) => !before.includes(x));
  const removed = before.filter((x) => !after.includes(x));
  const parts: string[] = [];
  if (added.length > 0) parts.push(`added ${added.map((x) => `"${x}"`).join(', ')}`);
  if (removed.length > 0) parts.push(`removed ${removed.map((x) => `"${x}"`).join(', ')}`);
  return `${LABEL_LABELS}: ${parts.join('; ') || 'reordered'}.`;
}

// Edits the personal labels of a skill through the same preview and confirm steps as every other edit.
export function LabelsEditor({ skill }: { skill: SkillDetail }) {
  const queryClient = useQueryClient();
  const saved = skill.labels ?? [];
  const savedKey = saved.join('\n');
  const [labels, setLabels] = useState<string[]>(saved);
  const [baseKey, setBaseKey] = useState(savedKey);
  const [typed, setTyped] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [proposal, setProposal] = useState<SkillProposal | null>(null);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  // A saved change (here or elsewhere) becomes the new starting point.
  if (savedKey !== baseKey) {
    setBaseKey(savedKey);
    setLabels(saved);
  }

  const dirty = labels.join('\n') !== savedKey;

  const showError = (err: unknown) => {
    if (err instanceof ApiError) {
      setErrorMessage(err.render.WHY || err.render.ERROR || err.message);
    } else if (err instanceof Error) {
      setErrorMessage(err.message);
    }
  };

  const handleAdd = () => {
    setLabels(addLabels(labels, typed));
    setTyped('');
  };

  const handlePreview = async () => {
    setLoading(true);
    setErrorMessage(null);
    try {
      // Labels live in the skill metadata, not in SKILL.md, so no content digest is pinned.
      setProposal(await previewSkillUpdate(skill.skill_id, { labels }));
    } catch (err) {
      showError(err);
    } finally {
      setLoading(false);
    }
  };

  const handleConfirm = async (pins: { proposalId: string; proposalDigest: string; baseVersion: string }) => {
    try {
      const done = await confirmSkillMutation(pins.proposalId, pins);
      setProposal(null);
      setToastMessage(`${skill.skill_id} labels committed. ${commitNote(done.commit)}`);
      queryClient.invalidateQueries({ queryKey: ['skill', skill.skill_id] });
      queryClient.invalidateQueries({ queryKey: ['skills'] });
    } catch (err) {
      setProposal(null);
      showError(err);
    }
  };

  const pins = proposal?.confirmation?.confirmation?.pins ?? proposal?.confirmation?.pins ?? null;

  return (
    <div className="fg-fact">
      <div className="fg-fact__label">
        <label htmlFor="edit-labels">{LABEL_LABELS}</label>
      </div>
      <div className="fg-fact__value" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)', minWidth: 0 }}>
        {labels.length === 0 ? (
          <span style={{ color: 'var(--color-text-muted)' }}>{LABEL_NONE}</span>
        ) : (
          <ul aria-label={LABEL_CURRENT} style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-1)', listStyle: 'none', margin: 0, padding: 0 }}>
            {labels.map((label) => (
              <li key={label} className="fg-chip fg-chip--info" style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                <span>{label}</span>
                <button
                  type="button"
                  className="fg-icon-btn"
                  aria-label={`Remove label ${label}`}
                  style={{ border: 'none', background: 'transparent', padding: 0, cursor: 'pointer', lineHeight: 1 }}
                  onClick={() => setLabels(labels.filter((x) => x !== label))}
                >
                  ✕
                </button>
              </li>
            ))}
          </ul>
        )}
        <form
          style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-2)' }}
          onSubmit={(e) => {
            e.preventDefault();
            handleAdd();
          }}
        >
          <input
            id="edit-labels"
            className="fg-input"
            style={{ flex: '1 1 160px', minWidth: 0 }}
            placeholder={PLACEHOLDER_ADD}
            maxLength={MAX_LABEL_LENGTH}
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
          />
          <button type="submit" className="fg-btn fg-btn--secondary fg-btn--small" disabled={!typed.trim()}>
            <span>{LABEL_ADD}</span>
          </button>
        </form>
        <span className="fg-field__hint">
          <span>{HINT_LABELS}</span>
        </span>
        {errorMessage && (
          <div className="fg-banner fg-banner--danger" role="alert">
            <span className="fg-banner__dot" />
            <span className="fg-banner__body">
              <span>{errorMessage}</span>
            </span>
          </div>
        )}
        {dirty && (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-2)' }}>
            <button type="button" className="fg-btn fg-btn--primary fg-btn--small" onClick={handlePreview} disabled={loading}>
              <span>{loading ? LABEL_VALIDATING : LABEL_PREVIEW}</span>
            </button>
            <button type="button" className="fg-btn fg-btn--secondary fg-btn--small" onClick={() => setLabels(saved)} disabled={loading}>
              <span>{LABEL_RESET}</span>
            </button>
          </div>
        )}
      </div>

      {proposal && (
        <ProposalPreview
          open
          title={`Commit label changes to ${skill.skill_id}?`}
          target={skill.skill_id}
          toState={skill.lifecycle_state}
          paths={proposalPaths(proposal)}
          changes={[describeLabelChange(saved, labels)]}
          warning={proposalWarning(proposal)}
          diff={proposalPatch(proposal)}
          proposalId={pins?.proposal_id || ''}
          proposalDigest={pins?.proposal_digest || ''}
          baseVersion={pins?.base_version || ''}
          confirmLabel="Commit changes"
          onConfirm={handleConfirm}
          onCancel={() => setProposal(null)}
        />
      )}

      <Toast message={toastMessage} onClose={() => setToastMessage(null)} />
    </div>
  );
}

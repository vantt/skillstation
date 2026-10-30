import type { HubCallEntry } from '../../api/types';

// What a call asked for: the tool of a tool call, otherwise the request kind.
export function requestLabel(call: { method?: string; tool?: string }): string {
  return call.tool ? call.tool : (call.method ?? '').replace('_', '/');
}

// The date and time of an RFC 3339 time in UTC, as "2026-10-30 11:59:31".
export function timeOfDay(at: string): string {
  return at.slice(0, 19).replace('T', ' ');
}

export interface SkillOutcome {
  skills: string[];
  // Only a skill_resolve call has a load and use to report.
  followed: { loaded: boolean; used: boolean } | null;
}

// The skills a call concerns: the one a skill_get names, or those a skill_resolve recommended and what followed.
export function skillOutcome(entry: HubCallEntry): SkillOutcome {
  const resolution = entry.resolution;
  if (!resolution) return { skills: entry.skill_id ? [entry.skill_id] : [], followed: null };
  const skills = resolution.recommended ?? [];
  return { skills, followed: skills.length > 0 ? { loaded: (resolution.loaded ?? []).length > 0, used: resolution.used } : null };
}

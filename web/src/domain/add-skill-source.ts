// Where the add screen takes skills from: a pasted GitHub URL or an uploaded zip.
// The choice lives in the `source` query parameter so a link can open the screen
// on either one and the page title follows it.
export type AddSource = 'github' | 'zip';

export const ADD_SOURCE_PARAM = 'source';

// parseAddSource falls back to GitHub for a missing or unknown value.
export function parseAddSource(value: string | null): AddSource {
  return value === 'zip' ? 'zip' : 'github';
}

export function addSkillPath(source: AddSource): string {
  return `/skills/add?${ADD_SOURCE_PARAM}=${source}`;
}

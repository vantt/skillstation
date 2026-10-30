// What a call asked for: the tool of a tool call, otherwise the request kind.
export function requestLabel(call: { method?: string; tool?: string }): string {
  return call.tool ? call.tool : (call.method ?? '').replace('_', '/');
}

// The date and time of an RFC 3339 time in UTC, as "2026-10-30 11:59:31".
export function timeOfDay(at: string): string {
  return at.slice(0, 19).replace('T', ' ');
}

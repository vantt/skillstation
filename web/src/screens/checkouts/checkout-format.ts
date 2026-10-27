// The date part of an RFC 3339 time, as other screens show dates.
export function checkoutDate(startedAt: string): string {
  return startedAt.slice(0, 10);
}

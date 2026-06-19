type ClassValue = string | number | false | null | undefined;

/** Tiny classnames joiner. Filters out falsy values. */
export function cn(...values: ClassValue[]): string {
  return values.filter(Boolean).join(' ');
}

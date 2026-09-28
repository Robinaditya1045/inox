import type { CSSProperties } from "react";

/**
 * Inline style that staggers a list item's entrance animation. Pair it with a
 * class that reads `animation-delay: calc(var(--i) * var(--stagger-step))`.
 * Capped so a long list does not keep its tail waiting on screen.
 */
export function stagger(index: number, max = 10): CSSProperties {
  return { "--i": Math.min(index, max) } as CSSProperties;
}

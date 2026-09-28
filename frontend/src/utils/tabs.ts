/** Element ids linking a tab to its panel (aria-controls / aria-labelledby). */
export function tabIds(prefix: string, id: string) {
  return { tab: `${prefix}-tab-${id}`, panel: `${prefix}-panel-${id}` };
}

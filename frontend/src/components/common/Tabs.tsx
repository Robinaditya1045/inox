import { type KeyboardEvent, type ReactNode, useRef } from "react";
import { tabIds } from "../../utils/tabs";
import styles from "./Tabs.module.css";

export interface TabItem<T extends string> {
  id: T;
  label: ReactNode;
  icon?: ReactNode;
  badge?: ReactNode;
  /** Render as the emphasised call-to-action tab (pill variant only) */
  accent?: boolean;
}

interface TabsProps<T extends string> {
  items: TabItem<T>[];
  value: T;
  onChange: (value: T) => void;
  ariaLabel: string;
  /** Prefix for the tab/panel ids; panels use tabIds(prefix, id).panel */
  idPrefix: string;
  variant?: "underline" | "pill";
  className?: string;
}

/**
 * WAI-ARIA tabs: one tab stop for the whole list, arrow keys (and Home/End)
 * move between tabs and select them.
 */
export function Tabs<T extends string>({
  items,
  value,
  onChange,
  ariaLabel,
  idPrefix,
  variant = "underline",
  className = "",
}: TabsProps<T>) {
  const listRef = useRef<HTMLDivElement | null>(null);

  const focusTab = (index: number) => {
    const tabs =
      listRef.current?.querySelectorAll<HTMLButtonElement>('[role="tab"]');
    tabs?.[index]?.focus();
    onChange(items[index].id);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const current = items.findIndex((item) => item.id === value);
    const last = items.length - 1;
    let next: number | null = null;

    if (e.key === "ArrowRight") next = current === last ? 0 : current + 1;
    else if (e.key === "ArrowLeft") next = current === 0 ? last : current - 1;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = last;

    if (next !== null) {
      e.preventDefault();
      focusTab(next);
    }
  };

  return (
    <div
      ref={listRef}
      role="tablist"
      aria-label={ariaLabel}
      className={`${styles.list} ${styles[variant]} ${className}`}
      onKeyDown={handleKeyDown}
    >
      {items.map((item) => {
        const selected = item.id === value;
        const ids = tabIds(idPrefix, item.id);
        return (
          <button
            key={item.id}
            type="button"
            role="tab"
            id={ids.tab}
            aria-selected={selected}
            aria-controls={ids.panel}
            tabIndex={selected ? 0 : -1}
            className={`${styles.tab} ${item.accent ? styles.tabAccent : ""}`}
            onClick={() => onChange(item.id)}
          >
            {item.icon && (
              <span className={styles.icon} aria-hidden="true">
                {item.icon}
              </span>
            )}
            {item.label}
            {item.badge}
          </button>
        );
      })}
    </div>
  );
}

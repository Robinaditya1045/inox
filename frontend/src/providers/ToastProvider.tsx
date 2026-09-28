import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { CheckCircle2, AlertCircle, Info, X } from "lucide-react";
import { ToastContext, type ToastOptions } from "../contexts/toast.context";
import styles from "../components/common/Toast.module.css";

interface ToastItem extends ToastOptions {
  id: number;
  leaving: boolean;
}

const DEFAULT_DURATION_MS = 4000;
const EXIT_MS = 160;
const MAX_VISIBLE = 3;

const TONE_ICON = {
  default: Info,
  success: CheckCircle2,
  danger: AlertCircle,
} as const;

/**
 * Transient confirmations ("Invite link copied") and background failures that
 * have no form to sit next to. Field-level errors stay inline with their field.
 */
export const ToastProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const nextId = useRef(0);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());
  // Toasts not yet dismissed, oldest first — which one to retire when full.
  const liveIds = useRef<number[]>([]);

  const dismiss = useCallback((id: number) => {
    if (!liveIds.current.includes(id)) return;
    liveIds.current = liveIds.current.filter((x) => x !== id);

    const pending = timers.current.get(id);
    if (pending) clearTimeout(pending);

    setToasts((prev) =>
      prev.map((t) => (t.id === id ? { ...t, leaving: true } : t)),
    );
    // Removed once its exit animation has had time to play.
    timers.current.set(
      id,
      setTimeout(() => {
        timers.current.delete(id);
        setToasts((prev) => prev.filter((t) => t.id !== id));
      }, EXIT_MS),
    );
  }, []);

  const toast = useCallback(
    (options: ToastOptions) => {
      const id = ++nextId.current;
      liveIds.current.push(id);
      setToasts((prev) => [...prev, { ...options, id, leaving: false }]);
      timers.current.set(
        id,
        setTimeout(() => dismiss(id), options.duration ?? DEFAULT_DURATION_MS),
      );
      // Over the limit, the oldest leaves the same way it would have anyway —
      // timer cleared, exit animation played — instead of vanishing mid-read.
      while (liveIds.current.length > MAX_VISIBLE) {
        dismiss(liveIds.current[0]);
      }
    },
    [dismiss],
  );

  useEffect(() => {
    const pending = timers.current;
    return () => {
      pending.forEach((timer) => clearTimeout(timer));
      pending.clear();
    };
  }, []);

  const value = useMemo(() => ({ toast, dismiss }), [toast, dismiss]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      {createPortal(
        <section
          className={styles.viewport}
          aria-label="Notifications"
          aria-live="polite"
          aria-relevant="additions"
        >
          {toasts.map((t) => {
            const tone = t.tone ?? "default";
            const Icon = TONE_ICON[tone];
            return (
              <div
                key={t.id}
                className={styles.toast}
                data-tone={tone}
                data-state={t.leaving ? "closed" : "open"}
              >
                <span className={styles.icon} aria-hidden="true">
                  <Icon size={18} />
                </span>
                <div className={styles.text}>
                  <span className={styles.title}>{t.title}</span>
                  {t.description && (
                    <span className={styles.description}>{t.description}</span>
                  )}
                </div>
                <button
                  type="button"
                  className={styles.close}
                  onClick={() => dismiss(t.id)}
                  aria-label="Dismiss notification"
                >
                  <X size={14} />
                </button>
              </div>
            );
          })}
        </section>,
        document.body,
      )}
    </ToastContext.Provider>
  );
};

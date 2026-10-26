import { createContext } from "react";

export type ToastTone = "default" | "success" | "danger";

export interface ToastAction {
  label: string;
  onClick: () => void;
  /** "primary" for what the toast is asking you to do. Defaults to "secondary". */
  variant?: "primary" | "secondary";
}

export interface ToastOptions {
  title: string;
  description?: string;
  tone?: ToastTone;
  /** Milliseconds before it dismisses itself. Defaults to 4000. */
  duration?: number;
  /**
   * Buttons under the text; choosing one dismisses the toast. A toast with
   * actions has no close button, since one of them is how you turn it down.
   */
  actions?: ToastAction[];
}

export interface ToastContextValue {
  /** Shows a toast and returns its id, for dismissing it early. */
  toast: (options: ToastOptions) => number;
  dismiss: (id: number) => void;
}

export const ToastContext = createContext<ToastContextValue | null>(null);

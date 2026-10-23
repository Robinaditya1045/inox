import * as React from "react"

export type ToastTone = "default" | "success" | "error"

export interface ToastOptions {
  tone?: ToastTone
  /** Milliseconds on screen; errors stay a little longer by default */
  duration?: number
}

export interface ToastApi {
  toast: (message: string, options?: ToastOptions) => void
}

export const ToastContext = React.createContext<ToastApi | null>(null)

/** Transient confirmations ("Changes saved", "Link copied"). Mounted in App.tsx. */
export function useToast(): ToastApi {
  const api = React.useContext(ToastContext)
  if (!api) throw new Error("useToast used outside ToastProvider")
  return api
}

import * as React from "react"
import { CheckCircle2, CircleAlert, X } from "lucide-react"
import { ToastContext, type ToastApi, type ToastOptions, type ToastTone } from "@/hooks/useToast"
import { cx } from "@/lib/utils"

interface ToastItem {
  id: number
  message: string
  tone: ToastTone
}

/** Holds the toast stack; bottom-left like Studio's snackbars, above everything. */
export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<ToastItem[]>([])
  const timers = React.useRef(new Map<number, ReturnType<typeof setTimeout>>())
  const nextId = React.useRef(1)

  const dismiss = React.useCallback((id: number) => {
    setItems(prev => prev.filter(t => t.id !== id))
    const timer = timers.current.get(id)
    if (timer) clearTimeout(timer)
    timers.current.delete(id)
  }, [])

  const api = React.useMemo<ToastApi>(() => ({
    toast: (message: string, options: ToastOptions = {}) => {
      const tone = options.tone ?? "default"
      const id = nextId.current++
      // Three at most: a burst of confirmations should not stack up the screen.
      setItems(prev => [...prev.slice(-2), { id, message, tone }])
      timers.current.set(id, setTimeout(() => dismiss(id), options.duration ?? (tone === "error" ? 6000 : 3500)))
    },
  }), [dismiss])

  React.useEffect(() => {
    const pending = timers.current
    return () => pending.forEach(clearTimeout)
  }, [])

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div
        aria-live="polite"
        className="pointer-events-none fixed bottom-4 left-4 right-4 sm:right-auto z-[70] flex flex-col gap-2 sm:max-w-sm"
      >
        {items.map(t => (
          <div
            key={t.id}
            role={t.tone === "error" ? "alert" : "status"}
            className={cx(
              "pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3 text-sm shadow-2xl animate-enter",
              t.tone === "error"
                ? "border-rose-500/30 bg-[#1c0f12] text-rose-100"
                : "border-zinc-700 bg-zinc-900 text-zinc-100",
            )}
          >
            {t.tone === "success" && <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-400" aria-hidden="true" />}
            {t.tone === "error" && <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-rose-400" aria-hidden="true" />}
            <span className="flex-1 leading-snug">{t.message}</span>
            <button
              type="button"
              onClick={() => dismiss(t.id)}
              aria-label="Dismiss"
              className="-mr-1 rounded p-0.5 text-zinc-400 hover:text-white cursor-pointer"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

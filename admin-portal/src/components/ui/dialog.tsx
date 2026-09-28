import * as React from "react"
import { cn } from "@/lib/utils"
import { X } from "lucide-react"

interface DialogProps {
  open?: boolean
  onOpenChange?: (open: boolean) => void
  children: React.ReactNode
}

const DialogContext = React.createContext<{
  open: boolean
  onOpenChange: (open: boolean) => void
}>({ open: false, onOpenChange: () => {} })

export function Dialog({ open: controlledOpen, onOpenChange, children }: DialogProps) {
  const [internalOpen, setInternalOpen] = React.useState(false)
  const open = controlledOpen !== undefined ? controlledOpen : internalOpen

  const handleOpenChange = (newVal: boolean) => {
    if (controlledOpen === undefined) setInternalOpen(newVal)
    onOpenChange?.(newVal)
  }

  return (
    <DialogContext.Provider value={{ open, onOpenChange: handleOpenChange }}>
      {children}
    </DialogContext.Provider>
  )
}

export function DialogTrigger({ children, asChild, ...props }: React.HTMLAttributes<HTMLDivElement> & { asChild?: boolean }) {
  const { onOpenChange } = React.useContext(DialogContext)
  return (
    <div onClick={() => onOpenChange(true)} className="inline-block cursor-pointer" {...props}>
      {children}
    </div>
  )
}

export function DialogContent({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  const { open, onOpenChange } = React.useContext(DialogContext)
  const panelRef = React.useRef<HTMLDivElement | null>(null)

  // Stays mounted after `open` turns false until the exit animation finishes,
  // so the dialog leaves the way it arrived instead of vanishing.
  const [present, setPresent] = React.useState(open)
  if (open && !present) setPresent(true)

  // Escape-to-close and background scroll locking: this dialog is hand-rolled
  // rather than built on Radix, so neither behaviour came for free. Without them
  // the only way out of a modal was the small × (or the overlay), and the page
  // behind it kept scrolling under the open dialog.
  React.useEffect(() => {
    if (!open) return

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onOpenChange(false)
    }
    document.addEventListener("keydown", onKeyDown)

    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = "hidden"

    return () => {
      document.removeEventListener("keydown", onKeyDown)
      document.body.style.overflow = previousOverflow
    }
  }, [open, onOpenChange])

  // Move focus onto the dialog itself on open, and hand it back to whatever
  // opened it on close. Deliberately not the first control inside: in the room
  // inspector that is "Terminate Room", one stray Enter away from firing.
  React.useEffect(() => {
    if (!open) return
    const opener = document.activeElement as HTMLElement | null
    panelRef.current?.focus()
    return () => opener?.focus?.()
  }, [open])

  if (!present) return null

  const state = open ? "open" : "closed"

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center sm:p-4">
      <div
        data-state={state}
        className="fixed inset-0 bg-black/80 backdrop-blur-sm data-[state=open]:animate-overlay-in data-[state=closed]:animate-overlay-out"
        onClick={() => onOpenChange(false)}
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
        data-state={state}
        onAnimationEnd={(e) => {
          if (e.target === e.currentTarget && !open) setPresent(false)
        }}
        className={cn(
          "relative z-50 grid w-full max-w-lg gap-4 border border-zinc-800 bg-[#111111] p-6 shadow-2xl rounded-t-xl sm:rounded-xl max-h-[92dvh] overflow-y-auto overscroll-contain outline-none data-[state=open]:animate-dialog-in data-[state=closed]:animate-dialog-out",
          className
        )}
        {...props}
      >
        {children}
        <button
          type="button"
          aria-label="Close dialog"
          onClick={() => onOpenChange(false)}
          className="absolute right-4 top-4 rounded-md p-1 opacity-70 transition-[opacity,background-color,color] duration-150 hover:opacity-100 hover:bg-zinc-800 text-zinc-400 hover:text-white cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400"
        >
          <X className="h-4 w-4" />
          <span className="sr-only">Close</span>
        </button>
      </div>
    </div>
  )
}

export function DialogHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex flex-col space-y-1.5 text-center sm:text-left", className)} {...props} />
}

export function DialogTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return <h2 className={cn("text-lg font-semibold leading-none tracking-tight text-white", className)} {...props} />
}

export function DialogDescription({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("text-sm text-zinc-400", className)} {...props} />
}

export function DialogFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex flex-col-reverse sm:flex-row sm:justify-end sm:space-x-2", className)} {...props} />
}

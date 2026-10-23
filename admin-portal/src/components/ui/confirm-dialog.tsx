import * as React from "react"
import { Loader2 } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

interface ConfirmDialogProps {
  open: boolean
  title: string
  description: React.ReactNode
  confirmLabel: string
  destructive?: boolean
  /** Asks for a tick before the confirm button unlocks, as Studio does for deletes */
  acknowledgement?: string
  onConfirm: () => Promise<void> | void
  onClose: () => void
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  destructive,
  acknowledgement,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  const [busy, setBusy] = React.useState(false)
  const [acknowledged, setAcknowledged] = React.useState(false)

  // Each opening starts unacknowledged.
  const [wasOpen, setWasOpen] = React.useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setAcknowledged(false)
  }

  const confirm = async () => {
    setBusy(true)
    try {
      await onConfirm()
      onClose()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="leading-relaxed">{description}</DialogDescription>
        </DialogHeader>
        {acknowledgement && (
          <label className="flex items-start gap-3 rounded-lg border border-zinc-800 bg-black/30 p-3 text-sm text-zinc-300 cursor-pointer">
            <input
              type="checkbox"
              checked={acknowledged}
              onChange={(e) => setAcknowledged(e.target.checked)}
              className="mt-0.5 h-4 w-4 accent-rose-500 cursor-pointer"
            />
            {acknowledgement}
          </label>
        )}
        <DialogFooter className="gap-2">
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button
            variant={destructive ? "destructive" : "emerald"}
            onClick={confirm}
            disabled={busy || (!!acknowledgement && !acknowledged)}
          >
            {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

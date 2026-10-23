import * as React from "react"
import { ImageOff, Loader2 } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

interface ThumbnailLinkDialogProps {
  open: boolean
  initialUrl?: string
  onCancel: () => void
  onApply: (url: string) => void
}

type PreviewState = "idle" | "loading" | "ok" | "error"

function isHttpUrl(value: string): boolean {
  try {
    const u = new URL(value)
    return (u.protocol === "http:" || u.protocol === "https:") && !!u.host
  } catch {
    return false
  }
}

/**
 * Points the thumbnail at an image hosted elsewhere. The image is not copied:
 * the library shows it from that address, so it has to stay up.
 */
export function ThumbnailLinkDialog({ open, initialUrl = "", onCancel, onApply }: ThumbnailLinkDialogProps) {
  const [url, setUrl] = React.useState(initialUrl)
  const [preview, setPreview] = React.useState<{ url: string; state: PreviewState }>({ url: "", state: "idle" })

  // Each opening starts from the current link.
  const [wasOpen, setWasOpen] = React.useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      setUrl(initialUrl)
      setPreview({ url: "", state: "idle" })
    }
  }

  const trimmed = url.trim()
  const valid = isHttpUrl(trimmed)
  const state: PreviewState = !valid ? "idle" : preview.url === trimmed ? preview.state : "loading"

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Use an image link</DialogTitle>
          <DialogDescription>
            Paste the address of a JPEG, PNG or WebP image. It is shown from where it is hosted, so use a link that will stay up.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (state === "ok") onApply(trimmed)
          }}
          className="space-y-4"
        >
          <label className="block">
            <span className="text-xs font-medium text-zinc-400">Image URL</span>
            <input
              autoFocus
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://example.com/poster.jpg"
              inputMode="url"
              className="mt-1.5 h-10 w-full rounded-lg border border-zinc-700 bg-black/40 px-3 text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-500 focus:outline-none"
            />
          </label>
          {trimmed && !valid && <p className="text-sm text-rose-400">Enter a full http:// or https:// address.</p>}

          <div className="relative aspect-video w-full overflow-hidden rounded-lg bg-zinc-950 ring-1 ring-zinc-800">
            {valid && (
              <img
                key={trimmed}
                src={trimmed}
                alt="Linked thumbnail preview"
                onLoad={() => setPreview({ url: trimmed, state: "ok" })}
                onError={() => setPreview({ url: trimmed, state: "error" })}
                className={state === "ok" ? "h-full w-full object-cover" : "hidden"}
              />
            )}
            {state !== "ok" && (
              <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-sm text-zinc-500">
                {state === "loading" && <Loader2 className="h-5 w-5 animate-spin" aria-label="Loading preview" />}
                {state === "error" && (
                  <>
                    <ImageOff className="h-6 w-6 text-rose-400" aria-hidden="true" />
                    <span className="text-rose-300">No image could be loaded from this link.</span>
                  </>
                )}
                {state === "idle" && "A preview appears here"}
              </div>
            )}
          </div>

          <DialogFooter className="gap-2">
            <Button type="button" variant="ghost" onClick={onCancel}>
              Cancel
            </Button>
            <Button type="submit" variant="emerald" disabled={state !== "ok"}>
              Use this image
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

import * as React from "react"
import { useNavigate } from "react-router-dom"
import { CircleAlert, Film, Link2, Loader2, UploadCloud } from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { useCatalog } from "@/lib/contexts"
import { useToast } from "@/hooks/useToast"
import { errorMessage } from "@/lib/api"
import { cx } from "@/lib/utils"

interface UploadDialogProps {
  open: boolean
  mode: "file" | "link"
  onModeChange: (mode: "file" | "link") => void
  onClose: () => void
}

// The API buffers multipart uploads up to 1 GB; anything bigger is PUT straight
// to object storage with a presigned URL, up to 3 GB.
const MULTIPART_LIMIT = 1 << 30
const DIRECT_LIMIT = 3 * (1 << 30)

function formatSize(bytes: number): string {
  if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(2)} GB`
  return `${(bytes / (1 << 20)).toFixed(1)} MB`
}

/**
 * "Upload videos": pick a file (or paste a link), give it a title, and land on
 * its details page to add a thumbnail and captions while it transcodes.
 */
export function UploadDialog({ open, mode, onModeChange, onClose }: UploadDialogProps) {
  const { uploadMedia, uploadDirectS3, registerExternal } = useCatalog()
  const { toast } = useToast()
  const navigate = useNavigate()
  const fileRef = React.useRef<HTMLInputElement>(null)

  const [file, setFile] = React.useState<File | null>(null)
  const [title, setTitle] = React.useState("")
  const [description, setDescription] = React.useState("")
  const [sourceUrl, setSourceUrl] = React.useState("")
  const [progress, setProgress] = React.useState<number | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const [dragOver, setDragOver] = React.useState(false)

  const busy = progress !== null

  const [wasOpen, setWasOpen] = React.useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      setFile(null)
      setTitle("")
      setDescription("")
      setSourceUrl("")
      setProgress(null)
      setError(null)
    }
  }

  const pick = (f: File | undefined) => {
    if (!f) return
    if (f.size > DIRECT_LIMIT) {
      setError(`${f.name} is ${formatSize(f.size)}; videos can be up to 3 GB.`)
      return
    }
    setError(null)
    setFile(f)
    if (!title) setTitle(f.name.replace(/\.[^/.]+$/, "").replace(/[_-]+/g, " ").trim())
  }

  const finish = (asset: MediaAsset, message: string) => {
    toast(message, { tone: "success" })
    onClose()
    navigate(`/content/${asset.id}/details`)
  }

  const submitFile = async () => {
    if (!file || !title.trim()) return
    setError(null)
    setProgress(0)
    try {
      const upload = file.size > MULTIPART_LIMIT ? uploadDirectS3 : uploadMedia
      const asset = await upload(file, title.trim(), description.trim(), (p) => setProgress(p))
      finish(asset, "Upload complete. The video is being processed — add a thumbnail and captions meanwhile.")
    } catch (err) {
      setError(errorMessage(err, "The upload failed."))
      setProgress(null)
    }
  }

  const submitLink = async () => {
    if (!title.trim() || !sourceUrl.trim()) return
    setError(null)
    setProgress(0)
    try {
      const asset = await registerExternal({ title: title.trim(), description: description.trim(), source_url: sourceUrl.trim() })
      finish(asset, "Video added to the library")
    } catch (err) {
      setError(errorMessage(err, "The link could not be added."))
      setProgress(null)
    }
  }

  const showDetails = mode === "link" || file !== null

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{mode === "file" ? "Upload video" : "Import from link"}</DialogTitle>
          <DialogDescription>
            {mode === "file"
              ? "Uploads are transcoded to adaptive HLS so every viewer gets a stream that suits their connection."
              : "Add an HLS playlist (.m3u8) or a video file hosted elsewhere. It plays from where it is hosted."}
          </DialogDescription>
        </DialogHeader>

        <div role="tablist" aria-label="Source" className="flex gap-5 border-b border-zinc-800">
          {(
            [
              ["file", "Upload file", UploadCloud],
              ["link", "Import from link", Link2],
            ] as const
          ).map(([value, label, Icon]) => (
            <button
              key={value}
              role="tab"
              type="button"
              aria-selected={mode === value}
              disabled={busy}
              onClick={() => {
                setError(null)
                onModeChange(value)
              }}
              className={cx(
                "-mb-px inline-flex items-center gap-2 border-b-2 pb-2.5 text-sm font-medium transition-colors cursor-pointer disabled:cursor-not-allowed",
                mode === value ? "border-white text-white" : "border-transparent text-zinc-400 hover:text-zinc-200",
              )}
            >
              <Icon className="h-4 w-4" aria-hidden="true" />
              {label}
            </button>
          ))}
        </div>

        {mode === "file" && !file && (
          <div
            onDragOver={(e) => {
              e.preventDefault()
              setDragOver(true)
            }}
            onDragLeave={() => setDragOver(false)}
            onDrop={(e) => {
              e.preventDefault()
              setDragOver(false)
              pick(e.dataTransfer.files[0])
            }}
            className={cx(
              "flex flex-col items-center rounded-xl border-2 border-dashed px-6 py-12 text-center transition-colors",
              dragOver ? "border-emerald-400 bg-emerald-500/5" : "border-zinc-800",
            )}
          >
            <div className="flex h-28 w-28 items-center justify-center rounded-full bg-zinc-900 ring-1 ring-zinc-800">
              <UploadCloud className={cx("h-12 w-12 transition-colors", dragOver ? "text-emerald-400" : "text-zinc-500")} aria-hidden="true" />
            </div>
            <p className="mt-5 text-sm text-zinc-200">Drag and drop a video file to upload</p>
            <p className="mt-1 text-xs text-zinc-500">MP4, MOV or MKV · up to 3 GB</p>
            <input ref={fileRef} type="file" accept="video/*,.mkv,.m3u8" className="hidden" onChange={(e) => pick(e.target.files?.[0])} />
            <Button variant="default" onClick={() => fileRef.current?.click()} className="mt-5">
              Select file
            </Button>
          </div>
        )}

        {showDetails && (
          <div className="space-y-4">
            {mode === "file" && file && (
              <div className="flex items-center gap-3 rounded-lg border border-zinc-800 bg-black/30 p-3">
                <div className="flex h-12 w-20 shrink-0 items-center justify-center rounded-md bg-zinc-900">
                  <Film className="h-5 w-5 text-zinc-500" aria-hidden="true" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-zinc-100">{file.name}</p>
                  <p className="text-xs text-zinc-500">
                    {formatSize(file.size)} · {file.size > MULTIPART_LIMIT ? "sent straight to storage" : "uploaded through the API"}
                  </p>
                </div>
                {!busy && (
                  <Button variant="ghost" size="sm" onClick={() => setFile(null)}>
                    Change
                  </Button>
                )}
              </div>
            )}

            {mode === "link" && (
              <Field label="Video URL (required)">
                <input
                  value={sourceUrl}
                  onChange={(e) => setSourceUrl(e.target.value)}
                  disabled={busy}
                  autoFocus
                  inputMode="url"
                  placeholder="https://cdn.example.com/show/master.m3u8"
                  className="h-10 w-full rounded-lg border border-zinc-700 bg-black/40 px-3 text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-500 focus:outline-none"
                />
              </Field>
            )}
            <Field label="Title (required)">
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                disabled={busy}
                maxLength={255}
                placeholder="Add a title that describes your video"
                className="h-10 w-full rounded-lg border border-zinc-700 bg-black/40 px-3 text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-500 focus:outline-none"
              />
            </Field>
            <Field label="Description">
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                disabled={busy}
                rows={3}
                maxLength={5000}
                placeholder="Tell viewers what the video is about"
                className="w-full resize-y rounded-lg border border-zinc-700 bg-black/40 px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-500 focus:outline-none"
              />
            </Field>
          </div>
        )}

        {busy && mode === "file" && (
          <div className="space-y-1.5" role="status" aria-live="polite">
            <div className="flex justify-between text-xs text-zinc-400">
              <span className="flex items-center gap-1.5">
                <Loader2 className="h-3.5 w-3.5 animate-spin text-emerald-400" aria-hidden="true" /> Uploading… keep this window open
              </span>
              <span className="tabular-nums text-zinc-200">{progress}%</span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-zinc-800">
              <div className="h-full rounded-full bg-emerald-500 transition-[width] duration-300 ease-snappy" style={{ width: `${progress}%` }} />
            </div>
          </div>
        )}

        {error && (
          <p role="alert" className="flex items-start gap-2 text-sm text-rose-300">
            <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" /> {error}
          </p>
        )}

        {showDetails && (
          <DialogFooter className="gap-2">
            <Button variant="ghost" onClick={onClose} disabled={busy}>
              Cancel
            </Button>
            <Button
              variant="emerald"
              onClick={mode === "file" ? submitFile : submitLink}
              disabled={busy || !title.trim() || (mode === "link" ? !sourceUrl.trim() : !file)}
            >
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {mode === "file" ? "Upload" : "Add to library"}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-xs font-medium text-zinc-400">{label}</span>
      <span className="mt-1.5 block">{children}</span>
    </label>
  )
}

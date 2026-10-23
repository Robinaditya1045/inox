import * as React from "react"
import { FileUp, Keyboard, Loader2, TriangleAlert } from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { LanguagePicker } from "@/components/content/LanguagePicker"
import { useCatalog } from "@/lib/contexts"
import { useToast } from "@/hooks/useToast"
import { errorMessage } from "@/lib/api"
import { languageName } from "@/lib/languages"
import { cx } from "@/lib/utils"

export interface ManualCaptionRequest {
  language: string
  label: string
  isDefault: boolean
}

interface AddCaptionsDialogProps {
  asset: MediaAsset
  open: boolean
  onClose: () => void
  /** "Type manually" hands over to the editor rather than saving anything yet */
  onTypeManually: (request: ManualCaptionRequest) => void
}

const MAX_CAPTION_FILE = 2 << 20

export function AddCaptionsDialog({ asset, open, onClose, onTypeManually }: AddCaptionsDialogProps) {
  const { createCaption } = useCatalog()
  const { toast } = useToast()
  const [language, setLanguage] = React.useState("")
  const [label, setLabel] = React.useState("")
  const [labelEdited, setLabelEdited] = React.useState(false)
  const [method, setMethod] = React.useState<"upload" | "manual">("upload")
  const [file, setFile] = React.useState<File | null>(null)
  const [isDefault, setIsDefault] = React.useState(false)
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [dragOver, setDragOver] = React.useState(false)

  const existing = asset.captions ?? []
  const replacing = existing.find((c) => c.language.toLowerCase() === language.toLowerCase())

  // Each opening starts blank; the first track defaults to being the default.
  const [wasOpen, setWasOpen] = React.useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      setLanguage("")
      setLabel("")
      setLabelEdited(false)
      setMethod("upload")
      setFile(null)
      setIsDefault(existing.length === 0)
      setError(null)
    }
  }

  const chooseLanguage = (tag: string) => {
    setLanguage(tag)
    if (!labelEdited) setLabel(tag ? languageName(tag) : "")
  }

  const pickFile = (f: File | undefined) => {
    if (!f) return
    if (!/\.(vtt|srt)$/i.test(f.name)) {
      setError("Choose a WebVTT (.vtt) or SubRip (.srt) file.")
      return
    }
    if (f.size > MAX_CAPTION_FILE) {
      setError("Caption files can be up to 2 MB.")
      return
    }
    setError(null)
    setFile(f)
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!language) {
      setError("Choose a language.")
      return
    }
    const trackLabel = label.trim() || languageName(language)
    if (method === "manual") {
      onTypeManually({ language, label: trackLabel, isDefault })
      return
    }
    if (!file) {
      setError("Choose a caption file to upload.")
      return
    }
    setBusy(true)
    setError(null)
    try {
      await createCaption(asset.id, { language, label: trackLabel, file, is_default: isDefault })
      toast(`${trackLabel} captions added`, { tone: "success" })
      onClose()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Add captions</DialogTitle>
          <DialogDescription>Upload a caption file or type them yourself. Viewers pick a language from the CC menu in the player.</DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-5">
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label htmlFor="caption-language" className="text-xs font-medium text-zinc-400">
                Language
              </label>
              <div className="mt-1.5">
                <LanguagePicker id="caption-language" value={language} onChange={chooseLanguage} taken={existing.map((c) => c.language)} />
              </div>
            </div>
            <label className="block">
              <span className="text-xs font-medium text-zinc-400">Name in the CC menu</span>
              <input
                value={label}
                onChange={(e) => {
                  setLabel(e.target.value)
                  setLabelEdited(true)
                }}
                maxLength={100}
                placeholder="e.g. English (SDH)"
                className="mt-1.5 h-10 w-full rounded-lg border border-zinc-700 bg-black/40 px-3 text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-500 focus:outline-none"
              />
            </label>
          </div>

          {replacing && (
            <p className="flex items-start gap-2 rounded-lg border border-amber-500/25 bg-amber-500/5 px-3 py-2 text-sm text-amber-200">
              <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-400" aria-hidden="true" />
              This video already has {replacing.label} captions. Saving replaces them.
            </p>
          )}

          <fieldset>
            <legend className="text-xs font-medium text-zinc-400">How do you want to add them?</legend>
            <div className="mt-1.5 grid gap-3 sm:grid-cols-2">
              <MethodCard
                checked={method === "upload"}
                onSelect={() => setMethod("upload")}
                icon={FileUp}
                title="Upload file"
                body="A .vtt or .srt file with timings, from an editor or a transcription service."
              />
              <MethodCard
                checked={method === "manual"}
                onSelect={() => setMethod("manual")}
                icon={Keyboard}
                title="Type manually"
                body="Watch the video and type captions as you go, timing them on a timeline."
              />
            </div>
          </fieldset>

          {method === "upload" && (
            <label
              onDragOver={(e) => {
                e.preventDefault()
                setDragOver(true)
              }}
              onDragLeave={() => setDragOver(false)}
              onDrop={(e) => {
                e.preventDefault()
                setDragOver(false)
                pickFile(e.dataTransfer.files[0])
              }}
              className={cx(
                "flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed px-4 py-6 text-center transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-emerald-400",
                dragOver ? "border-emerald-400 bg-emerald-500/5" : "border-zinc-700 hover:border-zinc-500",
              )}
            >
              <input type="file" accept=".vtt,.srt,text/vtt,application/x-subrip" className="sr-only" onChange={(e) => pickFile(e.target.files?.[0])} />
              <FileUp className="h-6 w-6 text-zinc-500" aria-hidden="true" />
              {file ? (
                <span className="mt-2 text-sm font-medium text-emerald-300">{file.name}</span>
              ) : (
                <span className="mt-2 text-sm text-zinc-300">Drop a .vtt or .srt file here, or click to choose</span>
              )}
              <span className="mt-1 text-xs text-zinc-500">UTF-8 text, up to 2 MB</span>
            </label>
          )}

          <label className="flex items-center gap-2.5 text-sm text-zinc-300 cursor-pointer">
            <input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} className="h-4 w-4 accent-emerald-500 cursor-pointer" />
            Default track — shown when a viewer turns captions on without choosing a language
          </label>

          {error && (
            <p role="alert" className="text-sm text-rose-300">
              {error}
            </p>
          )}

          <DialogFooter className="gap-2">
            <Button type="button" variant="ghost" onClick={onClose} disabled={busy}>
              Cancel
            </Button>
            <Button type="submit" variant="emerald" disabled={busy || !language || (method === "upload" && !file)}>
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {method === "manual" ? "Open editor" : "Upload"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function MethodCard({
  checked,
  onSelect,
  icon: Icon,
  title,
  body,
}: {
  checked: boolean
  onSelect: () => void
  icon: typeof FileUp
  title: string
  body: string
}) {
  return (
    <label
      className={cx(
        "flex cursor-pointer gap-3 rounded-lg border p-3 transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-emerald-400",
        checked ? "border-emerald-400/70 bg-emerald-500/[0.06]" : "border-zinc-800 hover:border-zinc-600",
      )}
    >
      <input type="radio" name="caption-method" checked={checked} onChange={onSelect} className="sr-only" />
      <Icon className={cx("mt-0.5 h-5 w-5 shrink-0", checked ? "text-emerald-400" : "text-zinc-500")} aria-hidden="true" />
      <span>
        <span className="block text-sm font-medium text-zinc-100">{title}</span>
        <span className="mt-0.5 block text-xs leading-relaxed text-zinc-400">{body}</span>
      </span>
    </label>
  )
}

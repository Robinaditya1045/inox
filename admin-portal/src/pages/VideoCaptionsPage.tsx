import * as React from "react"
import { useSearchParams } from "react-router-dom"
import { Captions, Download, EllipsisVertical, FileUp, Plus, Radio, Settings2, Star, StarOff, Trash2 } from "lucide-react"
import type { MediaAsset, MediaCaption } from "@/types/media"
import { PageBody, PageHeader } from "@/components/layout/PageHeader"
import { AddCaptionsDialog } from "@/components/content/AddCaptionsDialog"
import { CaptionEditor, type CaptionEditorTarget } from "@/components/content/CaptionEditor"
import { LanguagePicker } from "@/components/content/LanguagePicker"
import { Button } from "@/components/ui/button"
import { IconButton } from "@/components/ui/icon-button"
import { Menu } from "@/components/ui/menu"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { useCatalog, useVideoAsset } from "@/lib/contexts"
import { useToast } from "@/hooks/useToast"
import { errorMessage } from "@/lib/api"
import { formatDateTime, formatRelative } from "@/lib/format"
import { languageName } from "@/lib/languages"

export function VideoCaptionsPage() {
  const asset = useVideoAsset()
  const { updateCaption, deleteCaption } = useCatalog()
  const { toast } = useToast()
  const [params, setParams] = useSearchParams()

  // ?add=1 (from the details page's "Add captions") opens the dialog straight away.
  const [adding, setAdding] = React.useState(() => params.get("add") === "1")
  const [editing, setEditing] = React.useState<CaptionEditorTarget | null>(null)
  const [settingsFor, setSettingsFor] = React.useState<MediaCaption | null>(null)
  const [deleting, setDeleting] = React.useState<MediaCaption | null>(null)
  const replaceInput = React.useRef<HTMLInputElement>(null)
  const [replaceTarget, setReplaceTarget] = React.useState<MediaCaption | null>(null)

  React.useEffect(() => {
    if (params.get("add") !== "1") return
    const next = new URLSearchParams(params)
    next.delete("add")
    setParams(next, { replace: true })
  }, [params, setParams])

  const captions = asset.captions ?? []

  if (asset.kind === "live") {
    return (
      <PageBody wide>
        <PageHeader title="Video captions" />
        <div className="mt-10 flex flex-col items-center text-center">
          <Radio className="h-10 w-10 text-zinc-600" aria-hidden="true" />
          <p className="mt-4 text-sm font-medium text-zinc-200">Live channels can't have caption tracks</p>
          <p className="mt-1 max-w-md text-sm text-zinc-500">A live broadcast has no fixed timeline to time captions against.</p>
        </div>
      </PageBody>
    )
  }

  const toggleDefault = async (c: MediaCaption) => {
    try {
      await updateCaption(asset.id, c.id, { is_default: !c.is_default })
      toast(c.is_default ? `${c.label} is no longer the default` : `${c.label} is now the default track`, { tone: "success" })
    } catch (err) {
      toast(`Couldn't update: ${errorMessage(err)}`, { tone: "error" })
    }
  }

  const download = async (c: MediaCaption) => {
    try {
      const res = await fetch(c.url)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const blob = new Blob([await res.text()], { type: "text/vtt" })
      const a = document.createElement("a")
      a.href = URL.createObjectURL(blob)
      a.download = `${asset.title.replace(/[^\w\- ]+/g, "").trim() || "captions"}.${c.language}.vtt`
      a.click()
      setTimeout(() => URL.revokeObjectURL(a.href), 1000)
    } catch {
      toast("The caption file could not be downloaded.", { tone: "error" })
    }
  }

  const onReplaceFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ""
    const target = replaceTarget
    setReplaceTarget(null)
    if (!file || !target) return
    try {
      await updateCaption(asset.id, target.id, { file })
      toast(`${target.label} captions replaced`, { tone: "success" })
    } catch (err) {
      toast(`Couldn't replace: ${errorMessage(err)}`, { tone: "error" })
    }
  }

  return (
    <PageBody wide>
      <PageHeader
        title="Video captions"
        description="Caption tracks for this video. Viewers choose one from the CC menu in the room's player."
        actions={
          <Button variant="emerald" size="sm" onClick={() => setAdding(true)}>
            <Plus className="mr-1 h-3.5 w-3.5" aria-hidden="true" /> Add language
          </Button>
        }
      />

      <input ref={replaceInput} type="file" accept=".vtt,.srt,text/vtt,application/x-subrip" className="hidden" onChange={onReplaceFile} />

      {captions.length === 0 ? (
        <div className="mt-8 flex flex-col items-center rounded-xl border border-dashed border-zinc-800 px-6 py-16 text-center">
          <div className="flex h-20 w-20 items-center justify-center rounded-full bg-zinc-900 ring-1 ring-zinc-800">
            <Captions className="h-9 w-9 text-zinc-500" aria-hidden="true" />
          </div>
          <p className="mt-4 text-sm font-medium text-zinc-200">No captions yet</p>
          <p className="mt-1 max-w-md text-sm text-zinc-500">
            Captions let everyone follow along — in a noisy room, with the sound off, or in another language.
          </p>
          <Button variant="emerald" size="sm" onClick={() => setAdding(true)} className="mt-5">
            <Plus className="mr-1 h-3.5 w-3.5" aria-hidden="true" /> Add language
          </Button>
        </div>
      ) : (
        // relative: the header's sr-only label is absolutely positioned, and
        // would otherwise escape this scroller and widen the page on phones.
        <div className="relative mt-6 overflow-x-auto">
          <table className="w-full min-w-[640px] border-collapse text-left text-sm">
            <thead>
              <tr className="border-b border-zinc-800 text-xs text-zinc-400 [&>th]:py-3 [&>th]:font-medium">
                <th scope="col" className="pr-4">Language</th>
                <th scope="col" className="w-28 pr-4">Lines</th>
                <th scope="col" className="w-32 pr-4">Default</th>
                <th scope="col" className="w-40 pr-4">Last updated</th>
                <th scope="col" className="w-36 text-right">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {captions.map((c) => (
                <tr key={c.id} className="border-b border-zinc-800/70 hover:bg-zinc-900/60">
                  <td className="py-3 pr-4">
                    <span className="block font-medium text-zinc-100">{c.label}</span>
                    <span className="text-xs text-zinc-500">
                      {c.label !== languageName(c.language) && `${languageName(c.language)} · `}
                      <span className="font-mono">{c.language}</span>
                    </span>
                  </td>
                  <td className="py-3 pr-4 tabular-nums text-zinc-300">{c.cue_count}</td>
                  <td className="py-3 pr-4">
                    {c.is_default ? (
                      <span className="inline-flex items-center gap-1 text-emerald-300">
                        <Star className="h-3.5 w-3.5 fill-current" aria-hidden="true" /> Default
                      </span>
                    ) : (
                      <span className="text-zinc-600">—</span>
                    )}
                  </td>
                  <td className="py-3 pr-4 text-zinc-300" title={formatDateTime(c.updated_at)}>
                    {formatRelative(c.updated_at)}
                  </td>
                  <td className="py-3 text-right">
                    <div className="inline-flex items-center gap-1">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setEditing({ caption: c, language: c.language, label: c.label })}
                      >
                        Edit
                      </Button>
                      <Menu
                        label={`Options for ${c.label}`}
                        minWidth={230}
                        items={[
                          { label: "Download .vtt", icon: Download, onSelect: () => download(c) },
                          {
                            label: "Replace with a file…",
                            icon: FileUp,
                            onSelect: () => {
                              setReplaceTarget(c)
                              replaceInput.current?.click()
                            },
                          },
                          { label: "Name and language…", icon: Settings2, onSelect: () => setSettingsFor(c) },
                          {
                            label: c.is_default ? "Remove as default" : "Make default track",
                            icon: c.is_default ? StarOff : Star,
                            onSelect: () => toggleDefault(c),
                          },
                          { label: "Delete", icon: Trash2, destructive: true, separated: true, onSelect: () => setDeleting(c) },
                        ]}
                        trigger={(props) => (
                          <IconButton {...props} label="More options" size="sm">
                            <EllipsisVertical className="h-4 w-4" />
                          </IconButton>
                        )}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <AddCaptionsDialog
        asset={asset}
        open={adding}
        onClose={() => setAdding(false)}
        onTypeManually={({ language, label, isDefault }) => {
          setAdding(false)
          const existing = captions.find((c) => c.language.toLowerCase() === language.toLowerCase())
          // Typing into a language that already has a track edits that track.
          setEditing(existing ? { caption: existing, language: existing.language, label: existing.label } : { language, label, isDefault })
        }}
      />

      {editing && <CaptionEditor asset={asset} target={editing} onClose={() => setEditing(null)} />}

      <TrackSettingsDialog asset={asset} caption={settingsFor} onClose={() => setSettingsFor(null)} />

      <ConfirmDialog
        open={!!deleting}
        destructive
        title={`Delete ${deleting?.label ?? ""} captions?`}
        description="Viewers will no longer be able to choose this track. Download it first if you might want it back."
        confirmLabel="Delete"
        onConfirm={async () => {
          if (!deleting) return
          try {
            await deleteCaption(asset.id, deleting.id)
            toast(`${deleting.label} captions deleted`, { tone: "success" })
          } catch (err) {
            toast(`Couldn't delete: ${errorMessage(err)}`, { tone: "error" })
          }
        }}
        onClose={() => setDeleting(null)}
      />
    </PageBody>
  )
}

function TrackSettingsDialog({ asset, caption, onClose }: { asset: MediaAsset; caption: MediaCaption | null; onClose: () => void }) {
  const { updateCaption } = useCatalog()
  const { toast } = useToast()
  const [language, setLanguage] = React.useState("")
  const [label, setLabel] = React.useState("")
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  const [shownFor, setShownFor] = React.useState<string | null>(null)
  if (caption && caption.id !== shownFor) {
    setShownFor(caption.id)
    setLanguage(caption.language)
    setLabel(caption.label)
    setError(null)
  }

  const close = () => {
    setShownFor(null)
    onClose()
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!caption) return
    if (!language) {
      setError("Choose a language.")
      return
    }
    setBusy(true)
    try {
      await updateCaption(asset.id, caption.id, { language, label: label.trim() || languageName(language) })
      toast("Track updated", { tone: "success" })
      close()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={!!caption}
      onOpenChange={(o) => !o && !busy && close()}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Name and language</DialogTitle>
          <DialogDescription>How this track is labelled in the player's CC menu.</DialogDescription>
        </DialogHeader>
        {caption && (
          <form onSubmit={save} className="space-y-4">
            <div>
              <label htmlFor="track-language" className="text-xs font-medium text-zinc-400">
                Language
              </label>
              <div className="mt-1.5">
                <LanguagePicker
                  key={caption.id}
                  id="track-language"
                  value={language}
                  onChange={setLanguage}
                  taken={(asset.captions ?? []).filter((c) => c.id !== caption.id).map((c) => c.language)}
                />
              </div>
            </div>
            <label className="block">
              <span className="text-xs font-medium text-zinc-400">Name in the CC menu</span>
              <input
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                maxLength={100}
                className="mt-1.5 h-10 w-full rounded-lg border border-zinc-700 bg-black/40 px-3 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
              />
            </label>
            {error && (
              <p role="alert" className="text-sm text-rose-300">
                {error}
              </p>
            )}
            <DialogFooter className="gap-2">
              <Button type="button" variant="ghost" onClick={close} disabled={busy}>
                Cancel
              </Button>
              <Button type="submit" variant="emerald" disabled={busy || !language}>
                Save
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}

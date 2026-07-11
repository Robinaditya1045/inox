import * as React from "react"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Progress } from "@/components/ui/progress"
import { UploadCloud, Link2, Film, CheckCircle2, Loader2, AlertCircle, Zap } from "lucide-react"

interface VideoUploadModalProps {
  isOpen: boolean
  onClose: () => void
  onUploadFile: (file: File, title: string, description: string, onProgress?: (p: number) => void) => Promise<any>
  onUploadDirect: (file: File, title: string, description: string, onProgress?: (p: number) => void) => Promise<any>
  onRegisterExternal: (req: { title: string; description: string; source_url: string; thumbnail_url?: string }) => Promise<any>
}

export function VideoUploadModal({
  isOpen,
  onClose,
  onUploadFile,
  onUploadDirect,
  onRegisterExternal,
}: VideoUploadModalProps) {
  const [activeTab, setActiveTab] = React.useState("upload")
  const [file, setFile] = React.useState<File | null>(null)
  const [title, setTitle] = React.useState("")
  const [description, setDescription] = React.useState("")
  const [sourceUrl, setSourceUrl] = React.useState("")
  const [thumbnailUrl, setThumbnailUrl] = React.useState("")

  const [isSubmitting, setIsSubmitting] = React.useState(false)
  const [progress, setProgress] = React.useState(0)
  const [error, setError] = React.useState<string | null>(null)
  const [successMsg, setSuccessMsg] = React.useState<string | null>(null)

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const selected = e.target.files[0]
      setFile(selected)
      if (!title) setTitle(selected.name.replace(/\.[^/.]+$/, ""))
      setError(null)
    }
  }

  const handleUploadSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!file) {
      setError("Please select a video file (.mp4, .mov, or .m3u8 archive) to upload.")
      return
    }
    setIsSubmitting(true)
    setProgress(5)
    setError(null)
    setSuccessMsg(null)

    try {
      await onUploadFile(file, title, description, (p) => setProgress(p))
      setSuccessMsg("Video uploaded successfully! FFmpeg 3-tier ABR transcoding job initiated.")
      setTimeout(() => {
        resetForm()
        onClose()
      }, 1800)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed during transmission.")
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDirectSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!file) {
      setError("Please select a video file (up to 3GB) for direct S3/MinIO upload.")
      return
    }
    if (file.size > 3 * 1024 * 1024 * 1024) {
      setError("File exceeds the 3GB limit for direct storage upload.")
      return
    }
    setIsSubmitting(true)
    setProgress(5)
    setError(null)
    setSuccessMsg(null)

    try {
      await onUploadDirect(file, title, description, (p) => setProgress(p))
      setSuccessMsg("Direct S3 upload complete! File written directly to object storage & FFmpeg transcode queued.")
      setTimeout(() => {
        resetForm()
        onClose()
      }, 1800)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Direct storage transfer failed.")
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleRegisterSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title || !sourceUrl) {
      setError("Title and Source URL are required fields.")
      return
    }
    setIsSubmitting(true)
    setError(null)
    setSuccessMsg(null)

    try {
      await onRegisterExternal({
        title,
        description,
        source_url: sourceUrl,
        thumbnail_url: thumbnailUrl || undefined,
      })
      setSuccessMsg("External HLS stream cataloged successfully!")
      setTimeout(() => {
        resetForm()
        onClose()
      }, 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Registration failed.")
    } finally {
      setIsSubmitting(false)
    }
  }

  const resetForm = () => {
    setFile(null)
    setTitle("")
    setDescription("")
    setSourceUrl("")
    setThumbnailUrl("")
    setProgress(0)
    setError(null)
    setSuccessMsg(null)
  }

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && !isSubmitting && onClose()}>
      <DialogContent className="sm:max-w-[550px] border-zinc-800 bg-[#111111] text-white">
        <DialogHeader>
          <DialogTitle className="text-lg font-semibold flex items-center">
            <Film className="h-5 w-5 mr-2 text-emerald-400" /> Add Media Asset to Inox Catalog
          </DialogTitle>
          <DialogDescription className="text-xs text-zinc-400">
            Upload a physical video file for automated FFmpeg HLS transcoding or register an existing CDN stream.
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="upload" value={activeTab} onValueChange={setActiveTab} className="w-full mt-2">
          <TabsList className="grid w-full grid-cols-3 bg-zinc-900 border border-zinc-800">
            <TabsTrigger value="upload" className="text-xs font-mono truncate">
              <UploadCloud className="h-3.5 w-3.5 mr-1" /> Multipart (1GB)
            </TabsTrigger>
            <TabsTrigger value="direct" className="text-xs font-mono truncate text-cyan-400">
              <Zap className="h-3.5 w-3.5 mr-1" /> Direct S3 (3GB)
            </TabsTrigger>
            <TabsTrigger value="register" className="text-xs font-mono truncate">
              <Link2 className="h-3.5 w-3.5 mr-1" /> External CDN
            </TabsTrigger>
          </TabsList>

          {/* TAB 1: Multipart File Upload */}
          <TabsContent value="upload" className="space-y-4 pt-3">
            <form onSubmit={handleUploadSubmit} className="space-y-4">
              <div className="border-2 border-dashed border-zinc-800 hover:border-zinc-700 rounded-xl p-6 text-center bg-black/40 transition-colors cursor-pointer relative">
                <input
                  type="file"
                  accept="video/mp4,video/quicktime,application/x-mpegURL,.m3u8,.ts"
                  onChange={handleFileChange}
                  disabled={isSubmitting}
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                />
                <UploadCloud className="h-10 w-10 mx-auto text-zinc-500 mb-2 animate-bounce" style={{ animationDuration: "2s" }} />
                {file ? (
                  <div>
                    <p className="text-sm font-semibold text-emerald-400">{file.name}</p>
                    <p className="text-xs font-mono text-zinc-500 mt-1">
                      {(file.size / (1024 * 1024)).toFixed(2)} MB • {file.type || "video/mp4"}
                    </p>
                  </div>
                ) : (
                  <div>
                    <p className="text-sm font-medium text-zinc-300">Drag & drop video file or click to browse</p>
                    <p className="text-xs font-mono text-zinc-500 mt-1">Supports MP4, MOV, and HLS archives up to 1GB</p>
                  </div>
                )}
              </div>

              <div className="space-y-3">
                <div>
                  <Label htmlFor="title" className="text-xs font-mono text-zinc-400">ASSET TITLE *</Label>
                  <Input
                    id="title"
                    placeholder="e.g. Cyberpunk 2077 Gameplay Showcase"
                    value={title}
                    onChange={e => setTitle(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono"
                  />
                </div>

                <div>
                  <Label htmlFor="desc" className="text-xs font-mono text-zinc-400">DESCRIPTION (OPTIONAL)</Label>
                  <Input
                    id="desc"
                    placeholder="Brief description of video content or Watch Party notes..."
                    value={description}
                    onChange={e => setDescription(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono"
                  />
                </div>
              </div>

              {isSubmitting && (
                <div className="space-y-1.5 pt-2">
                  <div className="flex justify-between text-xs font-mono">
                    <span className="text-zinc-400 flex items-center">
                      <Loader2 className="h-3 w-3 mr-1.5 animate-spin text-emerald-400" /> Transmitting file to storage...
                    </span>
                    <span className="text-emerald-400 font-bold">{progress}%</span>
                  </div>
                  <Progress value={progress} className="h-1.5" />
                </div>
              )}

              {error && (
                <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs font-mono flex items-center">
                  <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" /> {error}
                </div>
              )}

              {successMsg && (
                <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-mono flex items-center">
                  <CheckCircle2 className="h-4 w-4 mr-2 flex-shrink-0" /> {successMsg}
                </div>
              )}

              <DialogFooter className="pt-2">
                <Button type="button" variant="ghost" onClick={onClose} disabled={isSubmitting} className="text-xs font-mono">
                  Cancel
                </Button>
                <Button type="submit" variant="emerald" disabled={isSubmitting || !file} className="text-xs font-mono">
                  {isSubmitting ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <UploadCloud className="h-3.5 w-3.5 mr-1.5" />}
                  Upload & Transcode
                </Button>
              </DialogFooter>
            </form>
          </TabsContent>

          {/* TAB 2: Direct Pre-Signed S3 Upload (Up to 3GB) */}
          <TabsContent value="direct" className="space-y-4 pt-3">
            <form onSubmit={handleDirectSubmit} className="space-y-4">
              <div className="border border-cyan-500/30 bg-cyan-950/20 rounded-lg p-3 text-xs font-mono text-cyan-300 flex items-start space-x-2">
                <Zap className="h-4 w-4 text-cyan-400 mt-0.5 flex-shrink-0" />
                <div>
                  <span className="font-semibold text-cyan-400">Server-RAM Bypass:</span> Direct chunked PUT transfer to MinIO / AWS S3 buckets. Ideal for IMAX feature films & 4K Watch Party assets up to <strong className="text-white">3,072 MB</strong>.
                </div>
              </div>

              <div className="border-2 border-dashed border-cyan-500/40 hover:border-cyan-400/60 rounded-xl p-6 text-center bg-cyan-950/10 transition-colors cursor-pointer relative">
                <input
                  type="file"
                  accept="video/mp4,video/quicktime,application/x-mpegURL,.m3u8,.ts,.mkv"
                  onChange={handleFileChange}
                  disabled={isSubmitting}
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                />
                <Zap className="h-10 w-10 mx-auto text-cyan-400 mb-2 animate-pulse" />
                {file ? (
                  <div>
                    <p className="text-sm font-semibold text-cyan-300">{file.name}</p>
                    <p className="text-xs font-mono text-cyan-500 mt-1">
                      {(file.size / (1024 * 1024)).toFixed(2)} MB • {file.type || "video/mp4"}
                    </p>
                  </div>
                ) : (
                  <div>
                    <p className="text-sm font-medium text-cyan-200">Drag & drop high-res video file or click to browse</p>
                    <p className="text-xs font-mono text-cyan-400/80 mt-1">Supports MP4, MOV, MKV up to 3 GB (3,072 MB)</p>
                  </div>
                )}
              </div>

              <div className="space-y-3">
                <div>
                  <Label htmlFor="s3-title" className="text-xs font-mono text-zinc-400">ASSET TITLE *</Label>
                  <Input
                    id="s3-title"
                    placeholder="e.g. Interstellar IMAX 4K Remaster"
                    value={title}
                    onChange={e => setTitle(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono text-cyan-300"
                  />
                </div>

                <div>
                  <Label htmlFor="s3-desc" className="text-xs font-mono text-zinc-400">DESCRIPTION (OPTIONAL)</Label>
                  <Input
                    id="s3-desc"
                    placeholder="High bitrate Watch Party source stream..."
                    value={description}
                    onChange={e => setDescription(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono"
                  />
                </div>
              </div>

              {isSubmitting && (
                <div className="space-y-1.5 pt-2">
                  <div className="flex justify-between text-xs font-mono">
                    <span className="text-cyan-400 flex items-center">
                      <Loader2 className="h-3 w-3 mr-1.5 animate-spin text-cyan-400" /> Streaming directly to S3 / MinIO bucket...
                    </span>
                    <span className="text-cyan-400 font-bold">{progress}%</span>
                  </div>
                  <Progress value={progress} className="h-1.5 bg-zinc-800" />
                </div>
              )}

              {error && (
                <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs font-mono flex items-center">
                  <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" /> {error}
                </div>
              )}

              {successMsg && (
                <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-mono flex items-center">
                  <CheckCircle2 className="h-4 w-4 mr-2 flex-shrink-0" /> {successMsg}
                </div>
              )}

              <DialogFooter className="pt-2">
                <Button type="button" variant="ghost" onClick={onClose} disabled={isSubmitting} className="text-xs font-mono">
                  Cancel
                </Button>
                <Button type="submit" variant="cyan" disabled={isSubmitting || !file} className="text-xs font-mono">
                  {isSubmitting ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <Zap className="h-3.5 w-3.5 mr-1.5" />}
                  Initiate Direct Upload
                </Button>
              </DialogFooter>
            </form>
          </TabsContent>

          {/* TAB 3: External Manifest Registration */}
          <TabsContent value="register" className="space-y-4 pt-3">
            <form onSubmit={handleRegisterSubmit} className="space-y-4">
              <div className="space-y-3">
                <div>
                  <Label htmlFor="ext-title" className="text-xs font-mono text-zinc-400">ASSET TITLE *</Label>
                  <Input
                    id="ext-title"
                    placeholder="e.g. Big Buck Bunny 4K HLS Stream"
                    value={title}
                    onChange={e => setTitle(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono"
                  />
                </div>

                <div>
                  <Label htmlFor="ext-url" className="text-xs font-mono text-zinc-400">HLS MASTER PLAYLIST URL (.M3U8 / .MP4) *</Label>
                  <Input
                    id="ext-url"
                    placeholder="https://cdn.example.com/hls/master.m3u8"
                    value={sourceUrl}
                    onChange={e => setSourceUrl(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono text-emerald-400"
                  />
                </div>

                <div>
                  <Label htmlFor="ext-thumb" className="text-xs font-mono text-zinc-400">THUMBNAIL IMAGE URL (OPTIONAL)</Label>
                  <Input
                    id="ext-thumb"
                    placeholder="https://images.unsplash.com/..."
                    value={thumbnailUrl}
                    onChange={e => setThumbnailUrl(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono"
                  />
                </div>

                <div>
                  <Label htmlFor="ext-desc" className="text-xs font-mono text-zinc-400">DESCRIPTION (OPTIONAL)</Label>
                  <Input
                    id="ext-desc"
                    placeholder="External CDN Watch Party asset..."
                    value={description}
                    onChange={e => setDescription(e.target.value)}
                    disabled={isSubmitting}
                    className="mt-1 bg-black/60 border-zinc-800 text-xs font-mono"
                  />
                </div>
              </div>

              {error && (
                <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs font-mono flex items-center">
                  <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" /> {error}
                </div>
              )}

              {successMsg && (
                <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-mono flex items-center">
                  <CheckCircle2 className="h-4 w-4 mr-2 flex-shrink-0" /> {successMsg}
                </div>
              )}

              <DialogFooter className="pt-2">
                <Button type="button" variant="ghost" onClick={onClose} disabled={isSubmitting} className="text-xs font-mono">
                  Cancel
                </Button>
                <Button type="submit" variant="cyan" disabled={isSubmitting || !title || !sourceUrl} className="text-xs font-mono">
                  {isSubmitting ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <Link2 className="h-3.5 w-3.5 mr-1.5" />}
                  Register External Stream
                </Button>
              </DialogFooter>
            </form>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}

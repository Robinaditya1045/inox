import * as React from "react"
import { Link } from "react-router-dom"
import { Captions, CircleAlert, CircleCheck, ImageOff, Loader2, UploadCloud } from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { PageBody, PageHeader } from "@/components/layout/PageHeader"
import { SystemPulseDashboard } from "@/components/dashboard/SystemPulseDashboard"
import { Thumb } from "@/components/content/Thumb"
import { StatusBadge } from "@/components/content/StatusBadge"
import { Button } from "@/components/ui/button"
import { useCatalog, useStudioActions, useTelemetry } from "@/lib/contexts"
import { formatRelative } from "@/lib/format"
import { cx } from "@/lib/utils"

export function DashboardPage() {
  const telemetry = useTelemetry()
  const { assets, isLoading } = useCatalog()
  const { openUpload } = useStudioActions()

  const videos = React.useMemo(
    () => assets.filter((a) => a.kind !== "live").sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()),
    [assets],
  )
  const stats = React.useMemo(() => {
    const ready = videos.filter((a) => a.status === "ready").length
    const processing = videos.filter((a) => a.status === "processing" || a.status === "pending").length
    const failed = videos.filter((a) => a.status === "failed").length
    const captioned = videos.filter((a) => (a.captions?.length ?? 0) > 0).length
    const noThumb = videos.filter((a) => a.status === "ready" && !a.thumbnail_url).length
    return { ready, processing, failed, captioned, noThumb, live: assets.length - videos.length }
  }, [videos, assets.length])

  const latest = videos[0]

  return (
    <PageBody wide>
      <PageHeader
        title="Dashboard"
        description="Your library at a glance, and how the platform is doing right now."
        actions={
          <Button variant="emerald" size="sm" onClick={() => openUpload("file")}>
            <UploadCloud className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" /> Upload video
          </Button>
        }
      />

      <div className="mt-6 grid gap-5 lg:grid-cols-3">
        <Card title="Latest upload">
          {isLoading ? (
            <div className="space-y-3">
              <div className="aspect-video animate-pulse rounded-lg bg-zinc-800/70" />
              <div className="h-4 w-2/3 animate-pulse rounded bg-zinc-800/70" />
            </div>
          ) : latest ? (
            <LatestUpload asset={latest} />
          ) : (
            <div className="flex flex-col items-center py-6 text-center">
              <UploadCloud className="h-10 w-10 text-zinc-600" aria-hidden="true" />
              <p className="mt-3 text-sm text-zinc-300">Nothing in the library yet</p>
              <p className="mt-1 text-sm text-zinc-500">Upload a video and rooms can start watching it together.</p>
              <Button variant="emerald" size="sm" className="mt-4" onClick={() => openUpload("file")}>
                Upload video
              </Button>
            </div>
          )}
        </Card>

        <Card title="Library" action={<CardLink to="/content">Go to content</CardLink>}>
          <div className="grid grid-cols-2 gap-4">
            <Stat label="Videos" value={videos.length} />
            <Stat label="Live channels" value={stats.live} />
          </div>
          <ul className="mt-5 space-y-2.5 text-sm">
            <StatusLine to="/content?status=ready" icon={CircleCheck} tone="text-emerald-400" label="Ready to play" value={stats.ready} />
            <StatusLine to="/content?status=processing" icon={Loader2} tone="text-amber-400" label="Processing" value={stats.processing} spin={stats.processing > 0} />
            <StatusLine to="/content?status=failed" icon={CircleAlert} tone="text-rose-400" label="Failed to process" value={stats.failed} />
          </ul>
          <div className="mt-5">
            <div className="flex justify-between text-xs text-zinc-400">
              <span>Caption coverage</span>
              <span className="tabular-nums">
                {stats.captioned} of {videos.length}
              </span>
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-zinc-800" aria-hidden="true">
              <div
                className="h-full rounded-full bg-emerald-500 transition-[width] duration-500 ease-snappy"
                style={{ width: `${videos.length ? (stats.captioned / videos.length) * 100 : 0}%` }}
              />
            </div>
          </div>
        </Card>

        <Card title="Needs attention">
          {stats.failed + (videos.length - stats.captioned) + stats.noThumb === 0 ? (
            <div className="flex flex-col items-center py-6 text-center">
              <CircleCheck className="h-9 w-9 text-emerald-400" aria-hidden="true" />
              <p className="mt-3 text-sm text-zinc-300">All caught up</p>
              <p className="mt-1 text-sm text-zinc-500">Every video is processed, captioned and has a thumbnail.</p>
            </div>
          ) : (
            <ul className="divide-y divide-zinc-800">
              {stats.failed > 0 && (
                <Attention to="/content?status=failed" icon={CircleAlert} tone="text-rose-400" title={`${stats.failed} failed to process`} body="Re-upload them, or delete them so rooms can't pick them." />
              )}
              {videos.length - stats.captioned > 0 && (
                <Attention
                  to="/content?captions=without"
                  icon={Captions}
                  tone="text-sky-400"
                  title={`${videos.length - stats.captioned} without captions`}
                  body="Captions help in noisy rooms, with the sound off, and across languages."
                />
              )}
              {stats.noThumb > 0 && (
                <Attention to="/content?status=ready" icon={ImageOff} tone="text-amber-400" title={`${stats.noThumb} without a thumbnail`} body="They show the default poster until one is captured or uploaded." />
              )}
            </ul>
          )}
        </Card>
      </div>

      {videos.length > 1 && (
        <Card title="Recent uploads" className="mt-5" action={<CardLink to="/content">See all</CardLink>}>
          <ul className="-mx-2 divide-y divide-zinc-800/70">
            {videos.slice(0, 5).map((a) => (
              <li key={a.id}>
                <Link to={`/content/${a.id}/details`} className="flex items-center gap-4 rounded-lg px-2 py-2.5 hover:bg-zinc-900">
                  <Thumb asset={a} className="w-24" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm text-zinc-100">{a.title}</span>
                    <span className="text-xs text-zinc-500">{formatRelative(a.created_at)}</span>
                  </span>
                  <span className="hidden sm:block">
                    <StatusBadge asset={a} />
                  </span>
                  <span className="hidden w-20 text-right text-xs text-zinc-500 md:block">
                    {(a.captions?.length ?? 0) > 0 ? `${a.captions!.length} caption track${a.captions!.length === 1 ? "" : "s"}` : "No captions"}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </Card>
      )}

      <h2 className="mb-4 mt-10 text-lg font-semibold text-white">Platform health</h2>
      <SystemPulseDashboard telemetry={telemetry} />
    </PageBody>
  )
}

function Card({ title, action, children, className }: { title: string; action?: React.ReactNode; children: React.ReactNode; className?: string }) {
  return (
    <section className={cx("rounded-xl border border-zinc-800 bg-surface p-5", className)}>
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-base font-medium text-white">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  )
}

function CardLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link to={to} className="text-sm font-medium text-emerald-400 hover:text-emerald-300">
      {children}
    </Link>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <p className="text-3xl font-semibold tabular-nums text-white">{value}</p>
      <p className="text-xs text-zinc-500">{label}</p>
    </div>
  )
}

function StatusLine({
  to,
  icon: Icon,
  tone,
  label,
  value,
  spin,
}: {
  to: string
  icon: typeof CircleCheck
  tone: string
  label: string
  value: number
  spin?: boolean
}) {
  return (
    <li>
      <Link to={to} className="flex items-center gap-2.5 rounded text-zinc-300 hover:text-white">
        <Icon className={cx("h-4 w-4", tone, spin && "animate-spin")} aria-hidden="true" />
        <span className="flex-1">{label}</span>
        <span className="tabular-nums text-zinc-400">{value}</span>
      </Link>
    </li>
  )
}

function Attention({ to, icon: Icon, tone, title, body }: { to: string; icon: typeof CircleCheck; tone: string; title: string; body: string }) {
  return (
    <li>
      <Link to={to} className="-mx-2 flex gap-3 rounded-lg px-2 py-3 hover:bg-zinc-900">
        <Icon className={cx("mt-0.5 h-4 w-4 shrink-0", tone)} aria-hidden="true" />
        <span>
          <span className="block text-sm font-medium text-zinc-100">{title}</span>
          <span className="mt-0.5 block text-xs leading-relaxed text-zinc-500">{body}</span>
        </span>
      </Link>
    </li>
  )
}

function LatestUpload({ asset }: { asset: MediaAsset }) {
  const tracks = asset.captions?.length ?? 0
  return (
    <div>
      <Link to={`/content/${asset.id}/details`} className="block">
        <Thumb asset={asset} className="w-full rounded-lg" />
      </Link>
      <Link to={`/content/${asset.id}/details`} className="mt-3 line-clamp-2 block text-sm font-medium text-zinc-100 hover:underline">
        {asset.title}
      </Link>
      <dl className="mt-3 space-y-2 text-sm">
        <div className="flex justify-between gap-3">
          <dt className="text-zinc-500">Status</dt>
          <dd>
            <StatusBadge asset={asset} />
          </dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-zinc-500">Captions</dt>
          <dd className="text-zinc-300">{tracks === 0 ? "None" : `${tracks} language${tracks === 1 ? "" : "s"}`}</dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-zinc-500">Added</dt>
          <dd className="text-zinc-300">{formatRelative(asset.created_at)}</dd>
        </div>
      </dl>
      <div className="mt-4 flex flex-wrap gap-2">
        <Link
          to={`/content/${asset.id}/details`}
          className="inline-flex h-8 items-center rounded-md border border-zinc-700 bg-zinc-900 px-3 text-xs font-semibold text-zinc-100 hover:bg-zinc-800"
        >
          Edit video
        </Link>
        {tracks === 0 && (
          <Link
            to={`/content/${asset.id}/captions?add=1`}
            className="inline-flex h-8 items-center rounded-md px-3 text-xs font-semibold text-emerald-400 hover:bg-emerald-500/10"
          >
            Add captions
          </Link>
        )}
      </div>
    </div>
  )
}

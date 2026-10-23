import { Link, Outlet, useParams } from "react-router-dom"
import { Film } from "lucide-react"
import { PageBody } from "@/components/layout/PageHeader"
import { useCatalog } from "@/lib/contexts"

/**
 * Resolves /content/:assetId for the pages under it. The catalog holds the whole
 * library (useMediaCatalog pages through it), so a video missing once it has
 * loaded has been deleted or never existed.
 */
export function VideoLayout() {
  const { assetId } = useParams()
  const { assets, isLoading } = useCatalog()
  const asset = assets.find((a) => a.id === assetId)

  if (!asset) {
    return (
      <PageBody>
        {isLoading ? (
          <div className="space-y-4" aria-busy="true" aria-label="Loading video">
            <div className="h-8 w-48 animate-pulse rounded bg-zinc-800/80" />
            <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_380px]">
              <div className="space-y-4">
                <div className="h-20 animate-pulse rounded-lg bg-zinc-900" />
                <div className="h-48 animate-pulse rounded-lg bg-zinc-900" />
              </div>
              <div className="aspect-video animate-pulse rounded-lg bg-zinc-900" />
            </div>
          </div>
        ) : (
          <div className="flex flex-col items-center py-24 text-center">
            <Film className="h-10 w-10 text-zinc-600" aria-hidden="true" />
            <h1 className="mt-4 text-lg font-medium text-white">This video isn't in the library</h1>
            <p className="mt-1 text-sm text-zinc-500">It may have been deleted.</p>
            <Link
              to="/content"
              className="mt-5 inline-flex h-8 items-center rounded-md border border-zinc-800 bg-zinc-900/60 px-3 text-xs font-semibold text-zinc-100 hover:border-zinc-700 hover:bg-zinc-800"
            >
              Back to content
            </Link>
          </div>
        )}
      </PageBody>
    )
  }

  return <Outlet context={{ asset }} />
}

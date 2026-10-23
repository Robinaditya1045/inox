import * as React from "react"
import { useOutletContext } from "react-router-dom"
import type { MediaAsset } from "@/types/media"
import type { UseTelemetryStreamResult } from "@/hooks/useTelemetryStream"
import type { UseMediaCatalogResult } from "@/hooks/useMediaCatalog"

// App owns these hooks and hands them down: routes mount and unmount as the
// operator moves around, and the telemetry socket (with its chart history) and
// the library (with its transcode polling) have to outlive any one page.

export const TelemetryContext = React.createContext<UseTelemetryStreamResult | null>(null)
export const CatalogContext = React.createContext<UseMediaCatalogResult | null>(null)

export interface StudioActions {
  /** Opens the upload dialog, on the tab for a file or for a link. */
  openUpload: (mode?: "file" | "link") => void
}
export const StudioActionsContext = React.createContext<StudioActions | null>(null)

function required<T>(value: T | null, name: string): T {
  if (value === null) throw new Error(`${name} used outside its provider (see App.tsx)`)
  return value
}

export function useTelemetry(): UseTelemetryStreamResult {
  return required(React.useContext(TelemetryContext), "useTelemetry")
}

export function useCatalog(): UseMediaCatalogResult {
  return required(React.useContext(CatalogContext), "useCatalog")
}

export function useStudioActions(): StudioActions {
  return required(React.useContext(StudioActionsContext), "useStudioActions")
}

/** The video a /content/:assetId/* page is about, resolved by VideoLayout. */
export function useVideoAsset(): MediaAsset {
  return useOutletContext<{ asset: MediaAsset }>().asset
}

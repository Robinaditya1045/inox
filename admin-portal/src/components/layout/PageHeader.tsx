import * as React from "react"
import { cx } from "@/lib/utils"

interface PageHeaderProps {
  title: string
  description?: React.ReactNode
  /** Buttons on the right; they wrap under the title on narrow screens */
  actions?: React.ReactNode
  className?: string
}

export function PageHeader({ title, description, actions, className }: PageHeaderProps) {
  return (
    <div className={cx("flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between", className)}>
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold tracking-tight text-white">{title}</h1>
        {description && <p className="mt-1 text-sm text-zinc-400">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}

/** The standard page gutter and width. */
export function PageBody({ children, className, wide }: { children: React.ReactNode; className?: string; wide?: boolean }) {
  return (
    <div className={cx("mx-auto w-full px-4 py-6 sm:px-6 lg:px-8", wide ? "max-w-[1600px]" : "max-w-7xl", className)}>
      {children}
    </div>
  )
}

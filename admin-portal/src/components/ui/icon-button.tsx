import * as React from "react"
import { cx } from "@/lib/utils"

export interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /** Accessible name, also shown as the tooltip */
  label: string
  /** Where the tooltip opens; "none" leaves just the accessible name */
  tooltip?: "top" | "bottom" | "none"
  size?: "sm" | "md"
  /** Classes for the wrapper, which is what sits in the parent's layout (e.g. hidden, ml-auto) */
  wrapperClassName?: string
  ref?: React.Ref<HTMLButtonElement>
}

/** A square icon-only button with a hover/focus tooltip. */
export function IconButton({
  label,
  tooltip = "top",
  size = "md",
  className,
  wrapperClassName,
  children,
  ref,
  ...props
}: IconButtonProps) {
  return (
    <span className={cx("group/tip relative inline-flex", wrapperClassName)}>
      <button
        ref={ref}
        type="button"
        aria-label={label}
        className={cx(
          "inline-flex items-center justify-center rounded-full text-zinc-300 transition-[color,background-color,transform] duration-150 ease-snappy hover:bg-zinc-800 hover:text-white active:scale-95 disabled:pointer-events-none disabled:opacity-40 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400",
          size === "sm" ? "h-8 w-8" : "h-10 w-10",
          className,
        )}
        {...props}
      >
        {children}
      </button>
      {tooltip !== "none" && (
        <span
          aria-hidden="true"
          className={cx(
            // display:none until shown, not just transparent: an absolutely
            // positioned tooltip still widens the page's scrollable area, which
            // made phones zoom out for buttons near the right edge.
            "pointer-events-none absolute left-1/2 z-30 hidden -translate-x-1/2 whitespace-nowrap rounded-md bg-zinc-700 px-2 py-1 text-xs font-medium text-white shadow-lg group-hover/tip:block group-has-[:focus-visible]/tip:block",
            tooltip === "top" ? "bottom-full mb-1.5" : "top-full mt-1.5",
          )}
        >
          {label}
        </span>
      )}
    </span>
  )
}

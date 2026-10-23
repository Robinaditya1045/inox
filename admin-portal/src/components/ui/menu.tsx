import * as React from "react"
import { createPortal } from "react-dom"
import type { LucideIcon } from "lucide-react"
import { cx } from "@/lib/utils"

export interface MenuItem {
  label: string
  icon?: LucideIcon
  onSelect: () => void
  destructive?: boolean
  disabled?: boolean
  /** Draws a divider above this item */
  separated?: boolean
  hint?: string
}

interface MenuProps {
  /** Rendered as the trigger; receives the props that wire it to the menu */
  trigger: (props: {
    ref: React.Ref<HTMLButtonElement>
    onClick: (e: React.MouseEvent) => void
    "aria-haspopup": "menu"
    "aria-expanded": boolean
  }) => React.ReactNode
  items: MenuItem[]
  align?: "start" | "end"
  /** Accessible name for the menu itself */
  label: string
  minWidth?: number
}

/**
 * A dropdown menu. Portalled and fixed-positioned, so it is not clipped by the
 * content table's scroll container, and flipped above the trigger when there is
 * no room below. Arrow keys move between items; Escape and Tab close it.
 */
export function Menu({ trigger, items, align = "end", label, minWidth = 200 }: MenuProps) {
  const [open, setOpen] = React.useState(false)
  const [pos, setPos] = React.useState<{ top: number; left: number; up: boolean } | null>(null)
  const triggerRef = React.useRef<HTMLButtonElement | null>(null)
  const menuRef = React.useRef<HTMLDivElement | null>(null)

  const place = React.useCallback(() => {
    const el = triggerRef.current
    if (!el) return
    const r = el.getBoundingClientRect()
    const estimatedHeight = items.length * 40 + 16
    const up = r.bottom + estimatedHeight > window.innerHeight - 8 && r.top > estimatedHeight
    const left = align === "end" ? Math.max(8, r.right - minWidth) : Math.min(r.left, window.innerWidth - minWidth - 8)
    setPos({ top: up ? r.top - 4 : r.bottom + 4, left, up })
  }, [align, items.length, minWidth])

  const close = React.useCallback((refocus = true) => {
    setOpen(false)
    if (refocus) triggerRef.current?.focus()
  }, [])

  React.useEffect(() => {
    if (!open) return
    const onPointerDown = (e: PointerEvent) => {
      const t = e.target as Node
      if (menuRef.current?.contains(t) || triggerRef.current?.contains(t)) return
      close(false)
    }
    // The position is computed once; anything that moves the trigger closes it.
    const onMove = () => close(false)
    document.addEventListener("pointerdown", onPointerDown)
    window.addEventListener("resize", onMove)
    window.addEventListener("scroll", onMove, true)
    return () => {
      document.removeEventListener("pointerdown", onPointerDown)
      window.removeEventListener("resize", onMove)
      window.removeEventListener("scroll", onMove, true)
    }
  }, [open, close])

  React.useEffect(() => {
    if (open) menuRef.current?.querySelector<HTMLButtonElement>("[role=menuitem]:not(:disabled)")?.focus()
  }, [open, pos])

  const onKeyDown = (e: React.KeyboardEvent) => {
    const nodes = Array.from(menuRef.current?.querySelectorAll<HTMLButtonElement>("[role=menuitem]:not(:disabled)") ?? [])
    const i = nodes.indexOf(document.activeElement as HTMLButtonElement)
    if (e.key === "ArrowDown") {
      e.preventDefault()
      nodes[(i + 1) % nodes.length]?.focus()
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      nodes[(i - 1 + nodes.length) % nodes.length]?.focus()
    } else if (e.key === "Home") {
      e.preventDefault()
      nodes[0]?.focus()
    } else if (e.key === "End") {
      e.preventDefault()
      nodes[nodes.length - 1]?.focus()
    } else if (e.key === "Escape") {
      e.preventDefault()
      e.stopPropagation()
      close()
    } else if (e.key === "Tab") {
      close(false)
    }
  }

  return (
    <>
      {trigger({
        ref: triggerRef,
        onClick: (e) => {
          e.stopPropagation()
          if (open) {
            close(false)
          } else {
            place()
            setOpen(true)
          }
        },
        "aria-haspopup": "menu",
        "aria-expanded": open,
      })}
      {open && pos &&
        createPortal(
          <div
            ref={menuRef}
            role="menu"
            aria-label={label}
            onKeyDown={onKeyDown}
            onClick={(e) => e.stopPropagation()}
            style={{ top: pos.top, left: pos.left, minWidth, transform: pos.up ? "translateY(-100%)" : undefined }}
            className="fixed z-[60] rounded-lg border border-zinc-700/80 bg-zinc-900 py-1.5 shadow-2xl shadow-black/60 animate-enter"
          >
            {items.map((item) => {
              const Icon = item.icon
              return (
                <React.Fragment key={item.label}>
                  {item.separated && <div className="my-1.5 h-px bg-zinc-800" role="separator" />}
                  <button
                    type="button"
                    role="menuitem"
                    disabled={item.disabled}
                    onClick={() => {
                      close()
                      item.onSelect()
                    }}
                    className={cx(
                      "flex w-full items-center gap-3 px-3.5 py-2 text-left text-sm outline-none transition-colors duration-100 cursor-pointer disabled:cursor-not-allowed disabled:opacity-40 focus-visible:bg-zinc-800 hover:bg-zinc-800",
                      item.destructive ? "text-rose-400" : "text-zinc-200",
                    )}
                  >
                    {Icon && <Icon className="h-4 w-4 shrink-0 opacity-80" aria-hidden="true" />}
                    <span className="flex-1">{item.label}</span>
                    {item.hint && <span className="text-xs text-zinc-500">{item.hint}</span>}
                  </button>
                </React.Fragment>
              )
            })}
          </div>,
          document.body,
        )}
    </>
  )
}

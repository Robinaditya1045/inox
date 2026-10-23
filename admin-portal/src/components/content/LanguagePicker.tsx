import * as React from "react"
import { Check } from "lucide-react"
import { COMMON_LANGUAGES, isLanguageTag, languageName } from "@/lib/languages"
import { cx } from "@/lib/utils"

interface LanguagePickerProps {
  id: string
  value: string
  onChange: (tag: string) => void
  /** Tags this video already has, marked in the list */
  taken?: string[]
}

/**
 * A combobox over common caption languages that also takes any BCP 47 tag
 * typed in, e.g. "en-AU" or "yue".
 */
export function LanguagePicker({ id, value, onChange, taken = [] }: LanguagePickerProps) {
  const [query, setQuery] = React.useState(value ? `${languageName(value)} (${value})` : "")
  const [open, setOpen] = React.useState(false)
  const [active, setActive] = React.useState(0)
  const listId = `${id}-list`

  const q = query.trim().toLowerCase()
  const matches = React.useMemo(() => {
    const all = COMMON_LANGUAGES.map((tag) => ({ tag: tag as string, name: languageName(tag) }))
    const typed = query.trim()
    const filtered = q ? all.filter((l) => l.name.toLowerCase().includes(q) || l.tag.toLowerCase().startsWith(q)) : all
    // A well-formed tag that is not in the list can still be chosen as typed.
    if (isLanguageTag(typed) && !filtered.some((l) => l.tag.toLowerCase() === typed.toLowerCase())) {
      filtered.unshift({ tag: typed, name: languageName(typed) })
    }
    return filtered
  }, [q, query])

  const choose = (tag: string) => {
    onChange(tag)
    setQuery(`${languageName(tag)} (${tag})`)
    setOpen(false)
  }

  return (
    <div className="relative">
      <input
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={open && matches[active] ? `${id}-opt-${active}` : undefined}
        value={query}
        placeholder="Search languages or type a tag, e.g. en-AU"
        onFocus={(e) => {
          e.currentTarget.select()
          setOpen(true)
        }}
        onChange={(e) => {
          setQuery(e.target.value)
          setActive(0)
          setOpen(true)
          onChange("")
        }}
        onBlur={() => setTimeout(() => setOpen(false), 120)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") {
            e.preventDefault()
            setOpen(true)
            setActive((i) => Math.min(matches.length - 1, i + 1))
          } else if (e.key === "ArrowUp") {
            e.preventDefault()
            setActive((i) => Math.max(0, i - 1))
          } else if (e.key === "Enter" && open && matches[active]) {
            e.preventDefault()
            choose(matches[active].tag)
          } else if (e.key === "Escape" && open) {
            e.stopPropagation()
            setOpen(false)
          }
        }}
        className="h-10 w-full rounded-lg border border-zinc-700 bg-black/40 px-3 text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-500 focus:outline-none"
      />
      {open && matches.length === 0 && (
        <p className="absolute left-0 right-0 top-full z-10 mt-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2.5 text-sm text-zinc-400 shadow-2xl">
          No match. Type a language tag instead, e.g. <span className="font-mono text-zinc-300">en-AU</span>.
        </p>
      )}
      {open && matches.length > 0 && (
        <ul
          id={listId}
          role="listbox"
          className="absolute left-0 right-0 top-full z-10 mt-1 max-h-56 overflow-y-auto rounded-lg border border-zinc-700 bg-zinc-900 py-1 shadow-2xl"
        >
          {matches.map((l, i) => (
            <li
              key={l.tag}
              id={`${id}-opt-${i}`}
              role="option"
              aria-selected={l.tag === value}
              onMouseDown={(e) => {
                e.preventDefault()
                choose(l.tag)
              }}
              onMouseEnter={() => setActive(i)}
              className={cx("flex cursor-pointer items-center gap-2 px-3 py-2 text-sm", i === active ? "bg-zinc-800 text-white" : "text-zinc-300")}
            >
              <span className="flex-1">{l.name}</span>
              {taken.includes(l.tag) && <span className="text-[11px] text-amber-300">has captions</span>}
              <span className="font-mono text-xs text-zinc-500">{l.tag}</span>
              {l.tag === value && <Check className="h-3.5 w-3.5 text-emerald-400" aria-hidden="true" />}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

export function cn(...classes: (string | undefined | null | false | Record<string, boolean>)[]): string {
  const rawClasses: string[] = []

  for (const item of classes) {
    if (!item) continue
    if (typeof item === 'string') {
      const parts = item.trim().split(/\s+/)
      for (const p of parts) {
        if (p) rawClasses.push(p)
      }
    } else if (typeof item === 'object') {
      for (const [key, value] of Object.entries(item)) {
        if (value) {
          const parts = key.trim().split(/\s+/)
          for (const p of parts) {
            if (p) rawClasses.push(p)
          }
        }
      }
    }
  }

  // Deduplicate and override utility classes by category so later classes cleanly override earlier ones
  const colorPrefixes = ['black', 'white', 'transparent', 'current', 'zinc-', 'emerald-', 'cyan-', 'rose-', 'amber-', 'purple-', 'blue-', 'green-', 'red-', 'yellow-', 'indigo-', 'gray-', 'slate-', 'orange-']
  const sizePrefixes = ['xs', 'sm', 'base', 'lg', 'xl', '2xl', '3xl', '4xl', '5xl']

  const getCategory = (cls: string): string | null => {
    // Text Color vs Text Size
    if (cls.startsWith('text-')) {
      const sub = cls.slice(5)
      if (colorPrefixes.some(cp => sub.startsWith(cp))) return 'text-color'
      if (sizePrefixes.some(sp => sub === sp)) return 'text-size'
      return 'text-other'
    }
    if (cls.startsWith('hover:text-')) {
      const sub = cls.slice(11)
      if (colorPrefixes.some(cp => sub.startsWith(cp))) return 'hover:text-color'
      return 'hover:text-other'
    }
    // Background Color
    if (cls.startsWith('bg-')) {
      const sub = cls.slice(3)
      if (colorPrefixes.some(cp => sub.startsWith(cp))) return 'bg-color'
      return 'bg-other'
    }
    if (cls.startsWith('hover:bg-')) {
      const sub = cls.slice(9)
      if (colorPrefixes.some(cp => sub.startsWith(cp))) return 'hover:bg-color'
      return 'hover:bg-other'
    }
    // Border Color
    if (cls.startsWith('border-')) {
      const sub = cls.slice(7)
      if (colorPrefixes.some(cp => sub.startsWith(cp))) return 'border-color'
      if (/^\d+$/.test(sub)) return 'border-width'
      return 'border-other'
    }
    if (cls.startsWith('hover:border-')) {
      const sub = cls.slice(13)
      if (colorPrefixes.some(cp => sub.startsWith(cp))) return 'hover:border-color'
      return 'hover:border-other'
    }
    // Shadows and Rounding
    if (cls.startsWith('shadow-') || cls === 'shadow') return 'shadow'
    if (cls.startsWith('rounded-') || cls === 'rounded') return 'rounded'
    if (cls.startsWith('px-')) return 'px'
    if (cls.startsWith('py-')) return 'py'
    if (cls.startsWith('p-') && /^\d/.test(cls.slice(2))) return 'p'
    if (cls.startsWith('h-') && !cls.startsWith('h-full') && !cls.startsWith('h-auto')) return 'h-exact'
    if (cls.startsWith('w-') && !cls.startsWith('w-full') && !cls.startsWith('w-auto')) return 'w-exact'
    if (cls === 'font-normal' || cls === 'font-medium' || cls === 'font-semibold' || cls === 'font-bold') return 'font-weight'

    return null
  }

  const result: string[] = []
  const seenCategories = new Set<string>()

  // Process from right to left so rightmost class in any mutually exclusive category wins
  for (let i = rawClasses.length - 1; i >= 0; i--) {
    const cls = rawClasses[i]
    const category = getCategory(cls)
    if (category) {
      if (!seenCategories.has(category)) {
        seenCategories.add(category)
        result.unshift(cls)
      }
    } else {
      result.unshift(cls)
    }
  }

  return result.join(" ")
}

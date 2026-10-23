/**
 * Caption languages offered in the "Add language" picker. Anything else can be
 * typed as a BCP 47 tag; the backend accepts any well-formed one.
 */
export const COMMON_LANGUAGES = [
  "en", "en-US", "en-GB", "es", "es-419", "fr", "de", "it", "pt", "pt-BR", "nl", "sv", "no", "da", "fi",
  "pl", "cs", "ro", "hu", "el", "tr", "ru", "uk", "ar", "he", "fa", "hi", "bn", "ur", "ta", "te", "mr",
  "gu", "kn", "ml", "pa", "th", "vi", "id", "ms", "fil", "zh-Hans", "zh-Hant", "ja", "ko", "sw",
] as const

const displayNames = (() => {
  try {
    // "standard" reads "English (United States)" rather than "American English",
    // so regional variants sort and scan together in the picker.
    return new Intl.DisplayNames(["en"], { type: "language", languageDisplay: "standard" })
  } catch {
    return null
  }
})()

/** "pt-BR" → "Portuguese (Brazil)"; the tag itself when the browser has no name. */
export function languageName(tag: string): string {
  try {
    return displayNames?.of(tag) ?? tag
  } catch {
    return tag
  }
}

/** Mirrors the backend's check (media.CanonicalLanguageTag), for instant feedback. */
export function isLanguageTag(tag: string): boolean {
  return tag.length <= 35 && /^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$/.test(tag.trim())
}

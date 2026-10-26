/**
 * Community post bodies are plain text with two conventions: a blank line
 * starts a new paragraph, and lines starting with "- " or "* " are list items.
 * Mirrors frontend/src/utils/postBody.ts, so the editor's preview matches what
 * viewers see.
 */
export type PostBlock =
  { kind: "paragraph"; lines: string[] } | { kind: "list"; items: string[] };

const BULLET = /^\s*[-*]\s+/;

export function parsePostBody(body: string): PostBlock[] {
  const blocks: PostBlock[] = [];
  let current: PostBlock | null = null;

  for (const raw of body.replace(/\r\n?/g, "\n").split("\n")) {
    const line = raw.trimEnd();
    if (!line.trim()) {
      current = null;
      continue;
    }
    if (BULLET.test(line)) {
      if (current?.kind !== "list") {
        current = { kind: "list", items: [] };
        blocks.push(current);
      }
      current.items.push(line.replace(BULLET, ""));
    } else {
      if (current?.kind !== "paragraph") {
        current = { kind: "paragraph", lines: [] };
        blocks.push(current);
      }
      current.lines.push(line.trim());
    }
  }
  return blocks;
}

export type TextPart = { text: string; href?: string };

const URL_PATTERN = /https?:\/\/[^\s<>"]+/g;

/** Splits text around http(s) links. Trailing punctuation stays out of the link. */
export function linkify(text: string): TextPart[] {
  const parts: TextPart[] = [];
  let last = 0;
  for (const match of text.matchAll(URL_PATTERN)) {
    const url = match[0].replace(/[.,;:!?)\]]+$/, "");
    const start = match.index ?? 0;
    if (start > last) parts.push({ text: text.slice(last, start) });
    parts.push({ text: url, href: url });
    last = start + url.length;
  }
  if (last < text.length) parts.push({ text: text.slice(last) });
  return parts;
}

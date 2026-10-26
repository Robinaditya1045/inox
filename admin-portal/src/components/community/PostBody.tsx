import * as React from "react";
import { linkify, parsePostBody } from "@/lib/postBody";

function Text({ text }: { text: string }) {
  return (
    <>
      {linkify(text).map((part, i) =>
        part.href ? (
          <a
            key={i}
            href={part.href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-emerald-400 underline underline-offset-2 hover:decoration-2"
          >
            {part.text}
          </a>
        ) : (
          <React.Fragment key={i}>{part.text}</React.Fragment>
        ),
      )}
    </>
  );
}

/** A post body rendered the way the viewer's Community page renders it. */
export function PostBody({ body }: { body: string }) {
  const blocks = React.useMemo(() => parsePostBody(body), [body]);
  return (
    <div className="flex flex-col gap-3 text-sm leading-relaxed text-zinc-300 [overflow-wrap:anywhere]">
      {blocks.map((block, i) =>
        block.kind === "list" ? (
          <ul
            key={i}
            className="flex list-disc flex-col gap-1 pl-5 marker:text-zinc-600"
          >
            {block.items.map((item, j) => (
              <li key={j}>
                <Text text={item} />
              </li>
            ))}
          </ul>
        ) : (
          <p key={i}>
            {block.lines.map((line, j) => (
              <React.Fragment key={j}>
                {j > 0 && <br />}
                <Text text={line} />
              </React.Fragment>
            ))}
          </p>
        ),
      )}
    </div>
  );
}

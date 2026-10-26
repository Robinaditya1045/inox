import React, { useMemo } from "react";
import { linkify, parsePostBody } from "../../utils/postBody";
import styles from "./PostCard.module.css";

const Text: React.FC<{ text: string }> = ({ text }) => (
  <>
    {linkify(text).map((part, i) =>
      part.href ? (
        <a
          key={i}
          href={part.href}
          target="_blank"
          rel="noopener noreferrer"
          className={styles.link}
        >
          {part.text}
        </a>
      ) : (
        <React.Fragment key={i}>{part.text}</React.Fragment>
      ),
    )}
  </>
);

/** A post body: paragraphs and bullet lists, with bare links made clickable. */
export const PostBody: React.FC<{ body: string }> = ({ body }) => {
  const blocks = useMemo(() => parsePostBody(body), [body]);
  return (
    <div className={styles.body}>
      {blocks.map((block, i) =>
        block.kind === "list" ? (
          <ul key={i} className={styles.list}>
            {block.items.map((item, j) => (
              <li key={j}>
                <Text text={item} />
              </li>
            ))}
          </ul>
        ) : (
          <p key={i} className={styles.paragraph}>
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
};

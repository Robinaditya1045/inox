import React, { useState, useRef, useEffect } from "react";
import { useChat } from "../../hooks/useChat";
import { useAuth } from "../../hooks/useAuth";
import { usePermissions } from "../../hooks/usePermissions";
import { usePresence } from "../../hooks/usePresence";
import { MessageItem } from "./ChatMessageItem";
import { Spinner } from "../common/Spinner";
import { Send, Lock, Hash, ArrowDown } from "lucide-react";
import type { ChatMessage } from "../../types/chat";
import styles from "./ChatPanel.module.css";

interface ChatPanelProps {
  roomId: string | undefined;
}

const GROUPING_THRESHOLD_MS = 5 * 60 * 1000;

interface FeedRow {
  message: ChatMessage;
  isLeader: boolean;
  /** Set when this message starts a new calendar day. */
  dateLabel: string | null;
}

function dayKey(iso: string): string {
  return new Date(iso).toDateString();
}

function formatDayLabel(iso: string): string {
  const date = new Date(iso);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);

  if (date.toDateString() === today.toDateString()) return "Today";
  if (date.toDateString() === yesterday.toDateString()) return "Yesterday";
  return date.toLocaleDateString([], {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

/** Groups consecutive messages by the same user within a 5-minute window, and marks day breaks. */
function buildFeed(messages: ChatMessage[]): FeedRow[] {
  return messages.map((msg, i) => {
    const prev = i > 0 ? messages[i - 1] : null;
    const isNewDay =
      !prev || dayKey(prev.created_at) !== dayKey(msg.created_at);

    let isLeader = true;
    if (prev && !isNewDay) {
      const sameUser = prev.user_id === msg.user_id;
      const withinWindow =
        Math.abs(
          new Date(msg.created_at).getTime() -
            new Date(prev.created_at).getTime(),
        ) < GROUPING_THRESHOLD_MS;
      isLeader = !(sameUser && withinWindow);
    }

    return {
      message: msg,
      isLeader,
      dateLabel: isNewDay ? formatDayLabel(msg.created_at) : null,
    };
  });
}

export const ChatPanel: React.FC<ChatPanelProps> = ({ roomId }) => {
  const [inputText, setInputText] = useState("");
  const [showScrollBottom, setShowScrollBottom] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const feedRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  const { messages, isLoadingHistory, sendMessage } = useChat(roomId);
  const { user } = useAuth();
  const { members } = usePresence();
  const permissions = usePermissions();

  const feed = buildFeed(messages);

  const scrollToBottom = (behavior: ScrollBehavior = "smooth") => {
    messagesEndRef.current?.scrollIntoView({ behavior });
    setShowScrollBottom(false);
  };

  useEffect(() => {
    if (!showScrollBottom) {
      scrollToBottom("auto");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [messages.length]);

  const handleScroll = () => {
    if (!feedRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = feedRef.current;
    setShowScrollBottom(scrollHeight - scrollTop - clientHeight > 80);
  };

  const autoGrow = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 180)}px`;
  };

  const submit = () => {
    if (!inputText.trim() || !permissions.can_send_messages) return;
    sendMessage(inputText);
    setInputText("");
    if (textareaRef.current) textareaRef.current.style.height = "auto";
    scrollToBottom("smooth");
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    submit();
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  const canSend = inputText.trim().length > 0;

  return (
    <div className={styles.panel}>
      <div ref={feedRef} onScroll={handleScroll} className={styles.feed}>
        {isLoadingHistory ? (
          <div className={styles.centered}>
            <Spinner size={20} />
            <span className={styles.centeredText}>Loading messages…</span>
          </div>
        ) : (
          <>
            {/* Channel intro — the top of the channel, not an empty state */}
            <div className={styles.intro}>
              <div className={styles.introIcon}>
                <Hash size={32} />
              </div>
              <h2 className={styles.introTitle}>Welcome to #general</h2>
              <p className={styles.introBody}>
                This is the beginning of the #general channel.
                {messages.length === 0
                  ? " Say hello to get things started."
                  : ""}
              </p>
            </div>

            {feed.map(({ message, isLeader, dateLabel }) => (
              <React.Fragment key={message.id}>
                {dateLabel && (
                  <div className={styles.divider}>
                    <span className={styles.dividerRule} aria-hidden="true" />
                    <span className={styles.dividerLabel}>{dateLabel}</span>
                    <span className={styles.dividerRule} aria-hidden="true" />
                  </div>
                )}
                <MessageItem
                  message={message}
                  isOwn={message.user_id === user?.id}
                  isGroupLeader={isLeader}
                  authorRole={
                    members.find((m) => m.user_id === message.user_id)?.role
                  }
                  currentUsername={user?.username}
                />
              </React.Fragment>
            ))}
          </>
        )}
        <div ref={messagesEndRef} />
      </div>

      {showScrollBottom && (
        <button
          onClick={() => scrollToBottom("smooth")}
          aria-label="Scroll to newest messages"
          className={styles.jumpBtn}
        >
          <ArrowDown size={13} />
          Jump to present
        </button>
      )}

      <div className={styles.composerWrap}>
        {!permissions.can_send_messages ? (
          <div className={styles.restricted}>
            <Lock size={13} aria-hidden="true" />
            <span>Messaging restricted</span>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className={styles.composer}>
            <textarea
              ref={textareaRef}
              value={inputText}
              onChange={(e) => {
                setInputText(e.target.value);
                autoGrow();
              }}
              onKeyDown={handleKeyDown}
              rows={1}
              placeholder="Message #general"
              aria-label="Message #general"
              className={styles.input}
            />
            <button
              type="submit"
              disabled={!canSend}
              aria-label="Send message"
              title="Send"
              className={`${styles.sendBtn} ${canSend ? styles.sendBtnReady : ""}`}
            >
              <Send size={16} />
            </button>
          </form>
        )}
      </div>
    </div>
  );
};

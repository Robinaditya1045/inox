import React, { useCallback, useEffect, useRef, useState } from "react";
import { useFriends } from "../../hooks/useFriends";
import { Button } from "../common/Button";
import { TextField } from "../common/TextField";
import { Badge } from "../common/Badge";
import { Spinner } from "../common/Spinner";
import { FriendRow } from "./FriendRow";
import type { UserSearchResult } from "../../types";
import { Search, UserPlus, Check, Clock } from "lucide-react";
import styles from "../../pages/FriendsPage.module.css";

/** Matches the server's minimum query length, so the UI doesn't promise results
 *  for a single character that the API will never return. */
const MIN_QUERY = 2;
const SEARCH_DEBOUNCE_MS = 250;

export const AddFriendPanel: React.FC = () => {
  const {
    sendRequest,
    acceptRequest,
    incomingRequests,
    friendsError,
    clearFriendsError,
    searchUsers,
  } = useFriends();

  const [query, setQuery] = useState("");
  const [results, setResults] = useState<UserSearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  // Guards against a slow search for "al" landing after a later one for "alice".
  const searchSeqRef = useRef(0);

  const runSearch = useCallback(
    async (term: string) => {
      const trimmed = term.trim();
      const seq = ++searchSeqRef.current;
      if (trimmed.length < MIN_QUERY) {
        setResults([]);
        setIsSearching(false);
        return;
      }
      setIsSearching(true);
      const found = await searchUsers(trimmed);
      if (seq !== searchSeqRef.current) return;
      setResults(found);
      setIsSearching(false);
    },
    [searchUsers],
  );

  useEffect(() => {
    const timer = setTimeout(() => runSearch(query), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [query, runSearch]);

  const send = useCallback(
    async (username: string) => {
      setNotice(null);
      clearFriendsError();
      setIsSubmitting(true);
      try {
        const outcome = await sendRequest(username);
        setNotice(
          outcome.status === "accepted"
            ? `You and ${username} are now friends — they had already added you.`
            : `Friend request sent to ${username}.`,
        );
        setQuery("");
        setResults([]);
      } catch {
        /* friendsError carries the message */
      } finally {
        setIsSubmitting(false);
      }
    },
    [sendRequest, clearFriendsError],
  );

  const accept = useCallback(
    async (userId: string, username: string) => {
      const pending = incomingRequests.find((req) => req.user_id === userId);
      if (!pending) return;
      setNotice(null);
      clearFriendsError();
      try {
        await acceptRequest(pending.id);
        setNotice(`You and ${username} are now friends.`);
        await runSearch(query);
      } catch {
        /* friendsError carries the message */
      }
    },
    [incomingRequests, acceptRequest, clearFriendsError, runSearch, query],
  );

  return (
    <div>
      <form
        className={styles.searchForm}
        onSubmit={(e) => {
          e.preventDefault();
          if (query.trim()) send(query.trim());
        }}
      >
        <TextField
          className={styles.searchField}
          label="Add a friend by username"
          placeholder="Enter an exact username, or search"
          value={query}
          icon={<Search size={14} />}
          onChange={(e) => {
            setQuery(e.target.value);
            setNotice(null);
            clearFriendsError();
          }}
          helperText="Usernames are case sensitive."
        />
        <Button
          type="submit"
          variant="primary"
          icon={<UserPlus size={14} />}
          isLoading={isSubmitting}
          disabled={!query.trim()}
        >
          Send Request
        </Button>
      </form>

      {friendsError && <div className={styles.error}>{friendsError}</div>}
      {notice && <div className={styles.notice}>{notice}</div>}

      <div style={{ marginTop: "var(--space-5)" }}>
        {isSearching && (
          <div className={styles.loading}>
            <Spinner size={16} />
            Searching…
          </div>
        )}

        {!isSearching && results.length > 0 && (
          <>
            <div className={styles.sectionLabel}>
              <Search size={12} /> Results
            </div>
            <div className={styles.list}>
              {results.map((result) => (
                <FriendRow
                  key={result.user_id}
                  username={result.username}
                  avatarUrl={result.avatar_url}
                >
                  {result.relationship === "friends" && (
                    <Badge variant="success">
                      <Check size={10} /> Friends
                    </Badge>
                  )}
                  {result.relationship === "request_sent" && (
                    <Badge variant="warning">
                      <Clock size={10} /> Requested
                    </Badge>
                  )}
                  {result.relationship === "request_received" && (
                    <Button
                      variant="primary"
                      size="sm"
                      icon={<Check size={12} />}
                      onClick={() => accept(result.user_id, result.username)}
                    >
                      Accept
                    </Button>
                  )}
                  {result.relationship === "none" && (
                    <Button
                      variant="secondary"
                      size="sm"
                      icon={<UserPlus size={12} />}
                      onClick={() => send(result.username)}
                    >
                      Add
                    </Button>
                  )}
                </FriendRow>
              ))}
            </div>
          </>
        )}

        {!isSearching &&
          query.trim().length >= MIN_QUERY &&
          results.length === 0 && (
            <div className={styles.empty}>
              <span className={styles.emptyTitle}>
                No one matches “{query.trim()}”
              </span>
              <span className={styles.emptyHint}>
                Check the spelling — usernames are case sensitive.
              </span>
            </div>
          )}
      </div>
    </div>
  );
};

import React, { useCallback, useEffect, useRef, useState } from "react";
import { useFriends } from "../../hooks/useFriends";
import { Button } from "../common/Button";
import { TextField } from "../common/TextField";
import { Badge } from "../common/Badge";
import { Spinner } from "../common/Spinner";
import { Alert } from "../common/Alert";
import { EmptyState } from "../common/EmptyState";
import { FriendRow } from "./FriendRow";
import type { UserSearchResult } from "../../types";
import { Search, UserPlus, Check, Clock, SearchX } from "lucide-react";
import { stagger } from "../../utils/motion";
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
    <>
      <div className={styles.intro}>
        <h2 className={styles.introTitle}>Add a friend</h2>
        <p className={styles.introText}>
          Send a request by exact username, or search and pick from the results.
        </p>
      </div>

      <form
        className={styles.searchForm}
        onSubmit={(e) => {
          e.preventDefault();
          if (query.trim()) send(query.trim());
        }}
      >
        <TextField
          className={styles.searchField}
          label="Username"
          name="friend-username"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          placeholder="Enter a username…"
          value={query}
          icon={<Search size={16} />}
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
          className={styles.searchSubmit}
          icon={<UserPlus size={14} />}
          isLoading={isSubmitting}
          disabled={!query.trim()}
        >
          Send Request
        </Button>
      </form>

      {friendsError && <Alert>{friendsError}</Alert>}
      {notice && <Alert tone="success">{notice}</Alert>}

      <div className={styles.results} aria-live="polite">
        {isSearching && (
          <div className={styles.searching} role="status">
            <Spinner size={14} label={null} />
            Searching…
          </div>
        )}

        {!isSearching && results.length > 0 && (
          <section aria-label="Search results">
            <div className={styles.sectionLabel}>
              <Search size={12} aria-hidden="true" /> Results — {results.length}
            </div>
            <div className={styles.list}>
              {results.map((result, i) => (
                <FriendRow
                  key={result.user_id}
                  username={result.username}
                  avatarUrl={result.avatar_url}
                  style={stagger(i)}
                >
                  {result.relationship === "friends" && (
                    <Badge variant="success">
                      <Check size={10} aria-hidden="true" /> Friends
                    </Badge>
                  )}
                  {result.relationship === "request_sent" && (
                    <Badge variant="warning">
                      <Clock size={10} aria-hidden="true" /> Requested
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
          </section>
        )}

        {!isSearching &&
          query.trim().length >= MIN_QUERY &&
          results.length === 0 && (
            <EmptyState
              compact
              icon={<SearchX size={20} />}
              title={<>No one matches “{query.trim()}”</>}
              description="Check the spelling — usernames are case sensitive."
            />
          )}
      </div>
    </>
  );
};

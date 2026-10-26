import { useEffect, useState } from "react";
import { communityService } from "../services/community/community.service";
import { isUnread, onSeenChange, readLastSeen } from "../utils/communitySeen";

/** How often the nav re-checks for new posts while the app is open. */
const POLL_MS = 5 * 60_000;

// The rail and the lobby sidebar both show the badge; they share one request.
let latest: { at: number; dates: Promise<(string | null)[]> } | null = null;

function latestDates(): Promise<(string | null)[]> {
  if (!latest || Date.now() - latest.at > POLL_MS / 2) {
    const dates = communityService
      .feed({ limit: 20 })
      .then((page) => page.posts.map((p) => p.published_at))
      .catch(() => {
        latest = null;
        return [];
      });
    latest = { at: Date.now(), dates };
  }
  return latest.dates;
}

/** Number of Community posts published since this browser last opened the page. */
export const useCommunityUnread = (): number => {
  const [count, setCount] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const recount = async () => {
      const dates = await latestDates();
      if (cancelled) return;
      const lastSeen = readLastSeen();
      setCount(dates.filter((at) => isUnread(at, lastSeen)).length);
    };

    recount();
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") recount();
    }, POLL_MS);
    const unsubscribe = onSeenChange(recount);
    return () => {
      cancelled = true;
      clearInterval(timer);
      unsubscribe();
    };
  }, []);

  return count;
};

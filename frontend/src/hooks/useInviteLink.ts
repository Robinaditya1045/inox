import { useCallback, useEffect, useRef, useState } from "react";
import { useToast } from "./useToast";

const COPIED_RESET_MS = 2500;

/** Copies a room's shareable URL and confirms it with a toast. */
export const useInviteLink = (roomId: string | undefined) => {
  const { toast } = useToast();
  const [copied, setCopied] = useState(false);
  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (resetTimer.current) clearTimeout(resetTimer.current);
    },
    [],
  );

  const copy = useCallback(() => {
    if (!roomId) return;
    const url = `${window.location.origin}/room/${roomId}`;

    // Without the clipboard API (plain http from a LAN address or a tunnel),
    // show the link so it can be copied by hand.
    const showUrl = () =>
      toast({
        tone: "danger",
        title: "Couldn't copy the link",
        description: url,
        duration: 8000,
      });

    if (!navigator.clipboard?.writeText) {
      showUrl();
      return;
    }

    navigator.clipboard
      .writeText(url)
      .then(() => {
        setCopied(true);
        toast({ tone: "success", title: "Invite link copied" });
        if (resetTimer.current) clearTimeout(resetTimer.current);
        resetTimer.current = setTimeout(
          () => setCopied(false),
          COPIED_RESET_MS,
        );
      })
      .catch(showUrl);
  }, [roomId, toast]);

  return { copied, copy };
};

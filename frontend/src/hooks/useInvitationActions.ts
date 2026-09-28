import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useRoom } from "./useRoom";
import { useToast } from "./useToast";
import type { RoomInvitation } from "../types";

/**
 * Accept / decline for pending room invitations, shared by the lobby page and
 * the lobby sidebar. Accepting joins and opens the room; either failure becomes
 * a toast rather than an unhandled rejection.
 */
export const useInvitationActions = () => {
  const { acceptInvitation, declineInvitation } = useRoom();
  const { toast } = useToast();
  const navigate = useNavigate();
  // Which invitation is being answered, and how, so only that button spins.
  const [busy, setBusy] = useState<{
    id: string;
    action: "accept" | "decline";
  } | null>(null);

  const roomLabel = (inv: RoomInvitation) => inv.room_name || "that room";

  const accept = useCallback(
    async (inv: RoomInvitation) => {
      setBusy({ id: inv.id, action: "accept" });
      try {
        const joined = await acceptInvitation(inv.id);
        navigate(`/room/${joined.id}`);
      } catch (err) {
        toast({
          tone: "danger",
          title: `Couldn't join ${roomLabel(inv)}`,
          description: err instanceof Error ? err.message : undefined,
        });
      } finally {
        setBusy(null);
      }
    },
    [acceptInvitation, navigate, toast],
  );

  const decline = useCallback(
    async (inv: RoomInvitation) => {
      setBusy({ id: inv.id, action: "decline" });
      try {
        await declineInvitation(inv.id);
      } catch (err) {
        toast({
          tone: "danger",
          title: `Couldn't decline the invite to ${roomLabel(inv)}`,
          description: err instanceof Error ? err.message : undefined,
        });
      } finally {
        setBusy(null);
      }
    },
    [declineInvitation, toast],
  );

  const isBusy = (inv: RoomInvitation, action?: "accept" | "decline") =>
    !!busy && busy.id === inv.id && (!action || busy.action === action);

  return { isBusy, accept, decline };
};

import { useContext } from "react";
import {
  PlayerSyncContext,
  type PlayerSyncContextValue,
} from "../contexts/playerSync.context";

export type PlayerSyncState = PlayerSyncContextValue;

export const usePlayerSync = (): PlayerSyncState => {
  const ctx = useContext(PlayerSyncContext);
  if (!ctx) {
    throw new Error("usePlayerSync must be used within a PlayerSyncProvider");
  }
  return ctx;
};

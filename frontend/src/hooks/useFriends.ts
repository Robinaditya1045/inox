import { useContext } from "react";
import {
  FriendsContext,
  type FriendsContextValue,
} from "../contexts/friends.context";

export const useFriends = (): FriendsContextValue => {
  const context = useContext(FriendsContext);
  if (!context) {
    throw new Error("useFriends must be used within a FriendsProvider");
  }
  return context;
};

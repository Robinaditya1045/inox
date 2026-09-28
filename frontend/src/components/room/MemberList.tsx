import React, { useState } from "react";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { usePresence } from "../../hooks/usePresence";
import { useAuth } from "../../hooks/useAuth";
import type { RoomRole } from "../../types/room";
import {
  Shield,
  UserMinus,
  Crown,
  ShieldAlert,
  UserCheck,
  MoreVertical,
  UserPlus,
  Check,
  AlertCircle,
  MicOff,
  ScreenShare,
  Volume2,
} from "lucide-react";
import { useRoom } from "../../hooks/useRoom";
import { useRTC } from "../../hooks/useRTC";
import { Avatar } from "../common/Avatar";
import { Button } from "../common/Button";
import { IconButton } from "../common/IconButton";
import { Spinner } from "../common/Spinner";
import { stagger } from "../../utils/motion";
import menuStyles from "../common/Menu.module.css";
import styles from "./MemberList.module.css";

const ROLE_LABEL: Record<RoomRole, string> = {
  owner: "Owner",
  moderator: "Moderator",
  member: "Member",
  guest: "Guest",
};

export const MemberList: React.FC = () => {
  const { members, isLoadingMembers, kickMember, updateRole, canModerate } =
    usePresence();
  const { user } = useAuth();
  const { permissions, inviteUser, activeRoom } = useRoom();
  const { peers: voicePeers, speakingIds } = useRTC(activeRoom?.id);

  const [inviteUsername, setInviteUsername] = useState("");
  const [isInviting, setIsInviting] = useState(false);
  const [inviteSuccess, setInviteSuccess] = useState<string | null>(null);
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [openMenuFor, setOpenMenuFor] = useState<string | null>(null);

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inviteUsername.trim()) return;
    setIsInviting(true);
    setInviteError(null);
    setInviteSuccess(null);
    try {
      await inviteUser(inviteUsername.trim());
      setInviteSuccess(`Invited @${inviteUsername.trim()}`);
      setInviteUsername("");
      setTimeout(() => setInviteSuccess(null), 4000);
    } catch (err) {
      setInviteError(
        err instanceof Error && err.message
          ? err.message
          : "Failed to invite user",
      );
    } finally {
      setIsInviting(false);
    }
  };

  const handleRoleChange = async (targetId: string, newRole: RoomRole) => {
    try {
      await updateRole(targetId, newRole);
    } catch {
      // Error handled in hook
    }
  };

  const handleKick = async (targetId: string) => {
    try {
      await kickMember(targetId);
    } catch {
      // Error handled in hook
    }
  };

  // Grouped the way a call-first client should read: who is in the call right
  // now, then everyone else by role. Membership comes from the server's voice
  // roster rather than from this client's own peer connection, which knows
  // nothing about people it has no media for.
  const voiceState = new Map(voicePeers.map((peer) => [peer.userId, peer]));

  const inVoice = members.filter((m) => voiceState.has(m.user_id));
  const rest = members.filter((m) => !voiceState.has(m.user_id));
  const rank: Record<RoomRole, number> = {
    owner: 0,
    moderator: 1,
    member: 2,
    guest: 3,
  };
  const byRank = [...rest].sort(
    (a, b) =>
      rank[a.role] - rank[b.role] || a.username.localeCompare(b.username),
  );

  const groups = [
    { key: "voice", label: "In Voice", members: inVoice },
    { key: "room", label: "In Room", members: byRank },
  ];

  return (
    <div className={styles.container}>
      {/* Invite Section for Private Rooms */}
      {(permissions?.can_invite_users || activeRoom?.is_private) && (
        <div className={styles.inviteSection}>
          <label className={styles.inviteHeader} htmlFor="member-invite-input">
            <UserPlus
              size={14}
              className={styles.inviteIcon}
              aria-hidden="true"
            />
            Invite to room
          </label>
          <form onSubmit={handleInvite} className={styles.inviteForm}>
            <input
              id="member-invite-input"
              type="text"
              name="invite-username"
              placeholder="Username…"
              autoComplete="off"
              spellCheck={false}
              value={inviteUsername}
              onChange={(e) => setInviteUsername(e.target.value)}
              disabled={isInviting}
              className={styles.inviteInput}
            />
            <Button
              type="submit"
              size="sm"
              isLoading={isInviting}
              disabled={!inviteUsername.trim()}
            >
              Invite
            </Button>
          </form>
          <div aria-live="polite">
            {inviteSuccess && (
              <span
                className={`${styles.inviteFeedback} ${styles.inviteSuccess}`}
              >
                <Check size={12} aria-hidden="true" /> {inviteSuccess}
              </span>
            )}
            {inviteError && (
              <span
                className={`${styles.inviteFeedback} ${styles.inviteError}`}
              >
                <AlertCircle size={12} aria-hidden="true" /> {inviteError}
              </span>
            )}
          </div>
        </div>
      )}

      {isLoadingMembers && (
        <div className={styles.updating} role="status">
          <Spinner size={12} label={null} />
          Updating…
        </div>
      )}

      <div className={styles.list}>
        {groups.map((group) =>
          group.members.length === 0 ? null : (
            <section
              key={group.key}
              className={styles.group}
              aria-label={`${group.label}, ${group.members.length}`}
            >
              <div className={styles.groupLabel} aria-hidden="true">
                {group.label} — {group.members.length}
              </div>
              {group.members.map((member, i) => {
                const isSelf = member.user_id === user?.id;
                const showAdminControls = !isSelf && canModerate(member);
                const voice = voiceState.get(member.user_id);
                const isSpeaking = speakingIds.has(member.user_id);
                const roleClass =
                  member.role === "owner"
                    ? styles.owner
                    : member.role === "moderator"
                      ? styles.moderator
                      : "";

                return (
                  <div
                    key={member.user_id}
                    className={`${styles.memberItem} ${roleClass}`}
                    style={stagger(i)}
                  >
                    <span
                      className={
                        isSpeaking ? styles.speakingAvatar : styles.idleAvatar
                      }
                    >
                      <Avatar
                        username={member.username}
                        src={isSelf ? user?.avatar_url : undefined}
                        size="sm"
                        status="online"
                      />
                    </span>

                    <div className={styles.memberDetails}>
                      <span className={styles.memberName}>
                        <span className={styles.nameText}>
                          {member.username}
                        </span>
                        {member.role === "owner" && (
                          <span className={styles.roleIcon}>
                            <Crown size={13} aria-label="Room owner" />
                          </span>
                        )}
                        {member.role === "moderator" && (
                          <span className={styles.roleIcon}>
                            <ShieldAlert size={13} aria-label="Moderator" />
                          </span>
                        )}
                        {isSelf && (
                          <span className={styles.selfLabel}>(you)</span>
                        )}
                        <span className={styles.stateIcons}>
                          {voice?.isMuted && (
                            <MicOff
                              size={12}
                              className={styles.muted}
                              aria-label="Muted"
                            />
                          )}
                          {isSpeaking && (
                            <Volume2
                              size={12}
                              className={styles.speaking}
                              aria-label="Speaking"
                            />
                          )}
                          {voice?.isScreenSharing && (
                            <ScreenShare
                              size={12}
                              className={styles.sharing}
                              aria-label="Sharing screen"
                            />
                          )}
                        </span>
                      </span>
                      <span className={styles.roleText}>
                        {ROLE_LABEL[member.role]}
                      </span>
                    </div>

                    {/* Moderation Actions Menu */}
                    {showAdminControls && (
                      <DropdownMenu.Root
                        onOpenChange={(open) =>
                          setOpenMenuFor(open ? member.user_id : null)
                        }
                      >
                        <span
                          className={styles.actionSlot}
                          data-open={
                            openMenuFor === member.user_id || undefined
                          }
                        >
                          <DropdownMenu.Trigger asChild>
                            <IconButton
                              label={`Manage ${member.username}`}
                              size="sm"
                              tooltip={false}
                            >
                              <MoreVertical size={16} />
                            </IconButton>
                          </DropdownMenu.Trigger>
                        </span>

                        <DropdownMenu.Portal>
                          <DropdownMenu.Content
                            className={menuStyles.content}
                            sideOffset={4}
                            align="end"
                          >
                            <DropdownMenu.Label className={menuStyles.label}>
                              Change role
                            </DropdownMenu.Label>

                            <DropdownMenu.Item
                              className={menuStyles.item}
                              onSelect={() =>
                                handleRoleChange(member.user_id, "moderator")
                              }
                            >
                              <span className={menuStyles.itemIcon}>
                                <ShieldAlert size={15} />
                              </span>
                              <span className={menuStyles.itemLabel}>
                                Promote to Mod
                              </span>
                              {member.role === "moderator" && (
                                <span className={menuStyles.itemCheck}>
                                  <Check size={14} />
                                </span>
                              )}
                            </DropdownMenu.Item>

                            <DropdownMenu.Item
                              className={menuStyles.item}
                              onSelect={() =>
                                handleRoleChange(member.user_id, "member")
                              }
                            >
                              <span className={menuStyles.itemIcon}>
                                <UserCheck size={15} />
                              </span>
                              <span className={menuStyles.itemLabel}>
                                Set as Member
                              </span>
                              {member.role === "member" && (
                                <span className={menuStyles.itemCheck}>
                                  <Check size={14} />
                                </span>
                              )}
                            </DropdownMenu.Item>

                            <DropdownMenu.Item
                              className={menuStyles.item}
                              onSelect={() =>
                                handleRoleChange(member.user_id, "guest")
                              }
                            >
                              <span className={menuStyles.itemIcon}>
                                <Shield size={15} />
                              </span>
                              <span className={menuStyles.itemLabel}>
                                Demote to Guest
                              </span>
                              {member.role === "guest" && (
                                <span className={menuStyles.itemCheck}>
                                  <Check size={14} />
                                </span>
                              )}
                            </DropdownMenu.Item>

                            <DropdownMenu.Separator
                              className={menuStyles.separator}
                            />

                            <DropdownMenu.Item
                              className={`${menuStyles.item} ${menuStyles.danger}`}
                              onSelect={() => handleKick(member.user_id)}
                            >
                              <span className={menuStyles.itemIcon}>
                                <UserMinus size={15} />
                              </span>
                              <span className={menuStyles.itemLabel}>
                                Kick from Room
                              </span>
                            </DropdownMenu.Item>
                          </DropdownMenu.Content>
                        </DropdownMenu.Portal>
                      </DropdownMenu.Root>
                    )}
                  </div>
                );
              })}
            </section>
          ),
        )}
      </div>
    </div>
  );
};

import React from "react";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { Check, ListPlus, Plus } from "lucide-react";
import type { MediaAsset, Playlist } from "../../types/media";
import menuStyles from "../common/Menu.module.css";
import styles from "./SaveToPlaylistMenu.module.css";

interface SaveToPlaylistMenuProps {
  asset: MediaAsset;
  playlists: Playlist[];
  onToggle: (playlistId: string, asset: MediaAsset) => void;
  onCreateNew: (asset: MediaAsset) => void;
  /** Extra class for the trigger, which sits over the poster */
  className?: string;
}

/** "Save to playlist": a checklist of the user's playlists, like YouTube's. */
export const SaveToPlaylistMenu: React.FC<SaveToPlaylistMenuProps> = ({
  asset,
  playlists,
  onToggle,
  onCreateNew,
  className = "",
}) => (
  <DropdownMenu.Root modal={false}>
    <DropdownMenu.Trigger asChild>
      <button
        type="button"
        className={`${styles.trigger} ${className}`}
        aria-label={`Save ${asset.title} to a playlist`}
        title="Save to playlist"
      >
        <ListPlus size={15} aria-hidden="true" />
      </button>
    </DropdownMenu.Trigger>
    <DropdownMenu.Portal>
      <DropdownMenu.Content
        className={`${menuStyles.content} ${styles.menu}`}
        side="bottom"
        align="end"
        sideOffset={6}
        collisionPadding={12}
      >
        <DropdownMenu.Label className={menuStyles.label}>
          Save to…
        </DropdownMenu.Label>
        {playlists.length === 0 && (
          <p className={styles.none}>You have no playlists yet.</p>
        )}
        <div className={styles.list}>
          {playlists.map((playlist) => {
            const saved = playlist.items.some(
              (item) => item.media_asset_id === asset.id,
            );
            return (
              <DropdownMenu.CheckboxItem
                key={playlist.id}
                className={menuStyles.item}
                checked={saved}
                // Stays open, so several playlists can be ticked in one go.
                onSelect={(e) => {
                  e.preventDefault();
                  onToggle(playlist.id, asset);
                }}
              >
                <span
                  className={styles.box}
                  data-checked={saved || undefined}
                  aria-hidden="true"
                >
                  {saved && <Check size={11} strokeWidth={3} />}
                </span>
                <span className={`${menuStyles.itemLabel} ${styles.name}`}>
                  {playlist.name}
                </span>
                <span className={styles.count}>{playlist.items.length}</span>
              </DropdownMenu.CheckboxItem>
            );
          })}
        </div>
        <DropdownMenu.Separator className={menuStyles.separator} />
        <DropdownMenu.Item
          className={menuStyles.item}
          onSelect={() => onCreateNew(asset)}
        >
          <span className={menuStyles.itemIcon}>
            <Plus size={15} />
          </span>
          <span className={menuStyles.itemLabel}>New playlist</span>
        </DropdownMenu.Item>
      </DropdownMenu.Content>
    </DropdownMenu.Portal>
  </DropdownMenu.Root>
);

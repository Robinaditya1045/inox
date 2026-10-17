import React, { useState } from "react";
import { Modal } from "../common/Modal";
import { TextField } from "../common/TextField";
import { Button } from "../common/Button";
import styles from "./PlaylistFormModal.module.css";

interface PlaylistFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  /** Present when editing; absent when creating */
  initial?: { name: string; description: string };
  /** Shown under the title, e.g. which video the new playlist will start with */
  context?: string;
  onSubmit: (name: string, description: string) => Promise<boolean>;
}

const MAX_NAME = 100;

/** Name (and optionally describe) a new playlist, or rename an existing one. */
export const PlaylistFormModal: React.FC<PlaylistFormModalProps> = ({
  isOpen,
  onClose,
  initial,
  context,
  onSubmit,
}) => {
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [isSaving, setIsSaving] = useState(false);
  const trimmed = name.trim();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!trimmed || isSaving) return;
    setIsSaving(true);
    const ok = await onSubmit(trimmed, description.trim());
    setIsSaving(false);
    if (ok) onClose();
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={initial ? "Edit playlist" : "New playlist"}
      description={context}
      maxWidth="420px"
    >
      <form className={styles.form} onSubmit={submit}>
        <TextField
          label="Name"
          name="playlist-name"
          autoComplete="off"
          autoFocus
          maxLength={MAX_NAME}
          placeholder="Friday night films"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <TextField
          label="Description (optional)"
          name="playlist-description"
          autoComplete="off"
          maxLength={1000}
          placeholder="What's it for?"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <div className={styles.actions}>
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={!trimmed} isLoading={isSaving}>
            {initial ? "Save" : "Create"}
          </Button>
        </div>
      </form>
    </Modal>
  );
};

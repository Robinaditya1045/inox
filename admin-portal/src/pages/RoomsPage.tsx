import * as React from "react";
import { useSearchParams } from "react-router-dom";
import {
  ArrowDown,
  ArrowUp,
  ChevronLeft,
  ChevronRight,
  Copy,
  EllipsisVertical,
  Filter,
  Globe,
  Lock,
  RefreshCw,
  Trash2,
  TriangleAlert,
  Users,
  X,
} from "lucide-react";
import type { AdminRoom } from "@/types/rooms";
import { PageBody, PageHeader } from "@/components/layout/PageHeader";
import { RoomCommandCenter } from "@/components/rooms/RoomCommandCenter";
import { FilterChip } from "@/components/content/FilterChip";
import { IconButton } from "@/components/ui/icon-button";
import { Menu } from "@/components/ui/menu";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { useAdminRooms } from "@/hooks/useAdminRooms";
import { useTelemetry } from "@/lib/contexts";
import { useToast } from "@/hooks/useToast";
import { errorMessage } from "@/lib/api";
import { formatDate, formatRelative } from "@/lib/format";
import { copyText, cx } from "@/lib/utils";

type View = "all" | "live";
type VisibilityFilter = "all" | "public" | "private";
type ActivityFilter = "all" | "active" | "idle";
type Sort =
  | "date-desc"
  | "date-asc"
  | "name-asc"
  | "name-desc"
  | "members-desc"
  | "members-asc";

const ROWS_KEY = "inox_admin_room_rows_per_page";
const ROW_OPTIONS = [10, 30, 50, 100] as const;

function readRowsPerPage(): number {
  try {
    const n = Number(localStorage.getItem(ROWS_KEY));
    return ROW_OPTIONS.includes(n as (typeof ROW_OPTIONS)[number]) ? n : 30;
  } catch {
    return 30;
  }
}

function ownerName(room: AdminRoom): string {
  const owner = room.members?.find(
    (m) => m.role === "owner" || m.user_id === room.owner_id,
  );
  return owner?.username || room.owner_id.slice(0, 8);
}

export function RoomsPage() {
  const [params, setParams] = useSearchParams();
  const view: View = params.get("view") === "live" ? "live" : "all";
  const telemetry = useTelemetry();

  return (
    <PageBody wide>
      <PageHeader
        title="Rooms"
        description="Every watch party on the server, public and private. Delete rooms, or inspect the ones in use."
      />

      <div
        role="tablist"
        aria-label="Rooms view"
        className="mt-6 flex gap-6 border-b border-zinc-800"
      >
        {(
          [
            ["all", "All rooms"],
            ["live", "Live inspector"],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            role="tab"
            type="button"
            aria-selected={view === value}
            onClick={() =>
              setParams(value === "live" ? { view: "live" } : {}, {
                replace: true,
              })
            }
            className={cx(
              "relative -mb-px border-b-2 px-1 pb-3 pt-1 text-sm font-medium transition-colors cursor-pointer",
              view === value
                ? "border-white text-white"
                : "border-transparent text-zinc-400 hover:text-zinc-200",
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {view === "live" ? (
        <div className="mt-6">
          <RoomCommandCenter liveRooms={telemetry.current?.rooms} />
        </div>
      ) : (
        <RoomManager />
      )}
    </PageBody>
  );
}

function RoomManager() {
  const { rooms, isLoading, isDemoMode, error, refresh, deleteRooms } =
    useAdminRooms();
  const { toast } = useToast();
  const [params, setParams] = useSearchParams();

  const query = params.get("q") ?? "";
  const visibility = (params.get("visibility") as VisibilityFilter) || "all";
  const activity = (params.get("activity") as ActivityFilter) || "all";
  const sort = (params.get("sort") as Sort) || "date-desc";

  const [page, setPage] = React.useState(0);
  const [rowsPerPage, setRowsPerPage] = React.useState(readRowsPerPage);
  const [selected, setSelected] = React.useState<Set<string>>(() => new Set());
  const [pendingDelete, setPendingDelete] = React.useState<AdminRoom[] | null>(
    null,
  );

  const setParam = (key: string, value: string, neutral: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === neutral) next.delete(key);
    else next.set(key, value);
    setParams(next, { replace: true });
    setPage(0);
  };

  const counts = React.useMemo(
    () => ({
      public: rooms.filter((r) => !r.is_private).length,
      private: rooms.filter((r) => r.is_private).length,
      active: rooms.filter((r) => r.activity).length,
    }),
    [rooms],
  );

  const rows = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = rooms.filter((r) => {
      if (visibility === "public" && r.is_private) return false;
      if (visibility === "private" && !r.is_private) return false;
      if (activity === "active" && !r.activity) return false;
      if (activity === "idle" && r.activity) return false;
      if (!q) return true;
      return (
        r.name.toLowerCase().includes(q) ||
        r.id.toLowerCase().includes(q) ||
        ownerName(r).toLowerCase().includes(q)
      );
    });
    const byDate = (r: AdminRoom) => new Date(r.created_at).getTime() || 0;
    const byMembers = (r: AdminRoom) => r.members?.length ?? 0;
    return filtered.sort((a, b) => {
      switch (sort) {
        case "date-asc":
          return byDate(a) - byDate(b);
        case "name-asc":
          return a.name.localeCompare(b.name);
        case "name-desc":
          return b.name.localeCompare(a.name);
        case "members-desc":
          return byMembers(b) - byMembers(a);
        case "members-asc":
          return byMembers(a) - byMembers(b);
        default:
          return byDate(b) - byDate(a);
      }
    });
  }, [rooms, query, visibility, activity, sort]);

  const pageCount = Math.max(1, Math.ceil(rows.length / rowsPerPage));
  const currentPage = Math.min(page, pageCount - 1);
  const pageRows = rows.slice(
    currentPage * rowsPerPage,
    (currentPage + 1) * rowsPerPage,
  );

  // Selection can span pages ("select all N"), but only ever names rows that
  // still exist and match the current filters.
  const selectedRows = rows.filter((r) => selected.has(r.id));
  const selectedOnPage = pageRows.filter((r) => selected.has(r.id)).length;
  const allOnPage = pageRows.length > 0 && selectedOnPage === pageRows.length;
  const allMatching = rows.length > 0 && selectedRows.length === rows.length;
  const headerCheckbox = React.useRef<HTMLInputElement>(null);
  React.useEffect(() => {
    if (headerCheckbox.current)
      headerCheckbox.current.indeterminate = selectedOnPage > 0 && !allOnPage;
  }, [selectedOnPage, allOnPage]);

  const toggleRow = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const togglePage = () =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (allOnPage) pageRows.forEach((r) => next.delete(r.id));
      else pageRows.forEach((r) => next.add(r.id));
      return next;
    });

  const filtersActive =
    query !== "" || visibility !== "all" || activity !== "all";
  const clearFilters = () => {
    setParams({}, { replace: true });
    setPage(0);
  };

  const copy = async (text: string, what: string) => {
    toast(
      (await copyText(text))
        ? `${what} copied`
        : "Could not copy to the clipboard",
    );
  };

  const runDelete = async (targets: AdminRoom[]) => {
    try {
      const deleted = await deleteRooms(targets.map((r) => r.id));
      setSelected(new Set());
      const skipped = targets.length - deleted.length;
      toast(
        deleted.length === 1
          ? `Deleted "${targets.find((r) => r.id === deleted[0])?.name ?? "room"}"`
          : `${deleted.length} rooms deleted`,
        { tone: "success" },
      );
      if (skipped > 0)
        toast(
          `${skipped} ${skipped === 1 ? "room was" : "rooms were"} already gone`,
        );
    } catch (err) {
      toast(`Could not delete: ${errorMessage(err)}`, { tone: "error" });
    }
  };

  const sortHeader = (
    label: string,
    asc: Sort,
    desc: Sort,
    className?: string,
  ) => {
    const active = sort === asc || sort === desc;
    return (
      <th
        scope="col"
        aria-sort={
          active ? (sort === asc ? "ascending" : "descending") : "none"
        }
        className={className}
      >
        <button
          type="button"
          onClick={() =>
            setParam("sort", sort === desc ? asc : desc, "date-desc")
          }
          className={cx(
            "inline-flex items-center gap-1 rounded hover:text-white cursor-pointer",
            active && "text-zinc-200",
          )}
        >
          {label}
          {active &&
            (sort === asc ? (
              <ArrowUp className="h-3.5 w-3.5" aria-hidden="true" />
            ) : (
              <ArrowDown className="h-3.5 w-3.5" aria-hidden="true" />
            ))}
        </button>
      </th>
    );
  };

  const pendingConnected =
    pendingDelete?.reduce((n, r) => n + (r.activity?.viewers ?? 0), 0) ?? 0;

  return (
    <>
      {(isDemoMode || error) && (
        <div
          role="status"
          className="mt-5 flex flex-wrap items-center gap-3 rounded-lg border border-amber-500/25 bg-amber-500/5 px-4 py-3 text-sm text-amber-200"
        >
          <TriangleAlert
            className="h-4 w-4 shrink-0 text-amber-400"
            aria-hidden="true"
          />
          <span className="flex-1">
            {error}
            {isDemoMode && " Deleting here changes nothing on the server."}
          </span>
          <Button variant="outline" size="sm" onClick={() => refresh()}>
            Retry
          </Button>
        </div>
      )}

      {/* Summary */}
      <div className="mt-5 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-zinc-400">
        <span>
          <span className="font-semibold tabular-nums text-white">
            {rooms.length}
          </span>{" "}
          rooms
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Globe className="h-3.5 w-3.5" aria-hidden="true" />
          <span className="tabular-nums text-zinc-200">
            {counts.public}
          </span>{" "}
          public
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Lock className="h-3.5 w-3.5" aria-hidden="true" />
          <span className="tabular-nums text-zinc-200">
            {counts.private}
          </span>{" "}
          private
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span
            className="h-2 w-2 rounded-full bg-emerald-400"
            aria-hidden="true"
          />
          <span className="tabular-nums text-zinc-200">{counts.active}</span> in
          use now
        </span>
        <Button
          variant="outline"
          size="sm"
          onClick={() => refresh()}
          className="sm:ml-auto"
        >
          <RefreshCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />{" "}
          Refresh
        </Button>
      </div>

      {/* Filter bar, or the bulk-action bar while rows are selected */}
      <div className="mt-3 flex min-h-14 flex-wrap items-center gap-2 border-y border-zinc-800 py-2.5">
        {selectedRows.length > 0 ? (
          <div className="flex w-full flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-zinc-800/70 px-3 py-1.5 animate-enter">
            <span className="text-sm font-medium text-white">
              {selectedRows.length} selected
            </span>
            {allOnPage && !allMatching && (
              <button
                type="button"
                onClick={() => setSelected(new Set(rows.map((r) => r.id)))}
                className="text-sm text-emerald-300 hover:underline cursor-pointer"
              >
                Select all {rows.length}{" "}
                {filtersActive ? "matching rooms" : "rooms"}
              </button>
            )}
            <span className="h-5 w-px bg-zinc-700" aria-hidden="true" />
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setPendingDelete(selectedRows)}
              className="text-rose-300 hover:text-rose-200"
            >
              <Trash2 className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />{" "}
              Delete forever
            </Button>
            <IconButton
              label="Clear selection"
              size="sm"
              wrapperClassName="ml-auto"
              onClick={() => setSelected(new Set())}
            >
              <X className="h-4 w-4" />
            </IconButton>
          </div>
        ) : (
          <>
            <label className="flex min-w-[220px] flex-1 items-center gap-2 text-sm">
              <Filter
                className="h-4 w-4 shrink-0 text-zinc-500"
                aria-hidden="true"
              />
              <span className="sr-only">Filter by room name, owner or ID</span>
              <input
                value={query}
                onChange={(e) => setParam("q", e.target.value, "")}
                placeholder="Filter by room name, owner or ID"
                className="h-9 w-full bg-transparent text-zinc-100 placeholder:text-zinc-500 focus:outline-none"
              />
            </label>
            <FilterChip
              label="Visibility"
              value={visibility}
              neutral="all"
              onChange={(v) => setParam("visibility", v, "all")}
              options={[
                { value: "all", label: "Any" },
                { value: "public", label: "Public" },
                { value: "private", label: "Private" },
              ]}
            />
            <FilterChip
              label="Activity"
              value={activity}
              neutral="all"
              onChange={(v) => setParam("activity", v, "all")}
              options={[
                { value: "all", label: "Any" },
                { value: "active", label: "In use now" },
                { value: "idle", label: "Empty" },
              ]}
            />
            {filtersActive && (
              <Button variant="ghost" size="sm" onClick={clearFilters}>
                Clear
              </Button>
            )}
          </>
        )}
      </div>

      {/* relative, so the sr-only labels and tooltips inside are clipped by this
          scroller instead of widening the page on phones. */}
      <div className="relative overflow-x-auto">
        <table className="w-full min-w-[860px] border-collapse text-left">
          <thead>
            <tr className="border-b border-zinc-800 text-xs font-medium text-zinc-400 [&>th]:py-3 [&>th]:font-medium">
              <th scope="col" className="w-12 pl-1">
                <input
                  ref={headerCheckbox}
                  type="checkbox"
                  aria-label="Select all on this page"
                  checked={allOnPage}
                  onChange={togglePage}
                  disabled={pageRows.length === 0}
                  className="h-4 w-4 accent-emerald-500 cursor-pointer"
                />
              </th>
              {sortHeader(
                "Room",
                "name-asc",
                "name-desc",
                "min-w-[300px] pr-4",
              )}
              <th scope="col" className="w-32 pr-4">
                Visibility
              </th>
              <th scope="col" className="w-40 pr-4">
                Owner
              </th>
              {sortHeader(
                "Members",
                "members-asc",
                "members-desc",
                "w-28 pr-4",
              )}
              <th scope="col" className="w-44 pr-4">
                Now
              </th>
              {sortHeader("Created", "date-asc", "date-desc", "w-36 pr-4")}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              Array.from({ length: 5 }, (_, i) => (
                <tr
                  key={i}
                  className="border-b border-zinc-800/70"
                  aria-hidden="true"
                >
                  <td className="py-4 pl-1">
                    <div className="h-4 w-4 rounded bg-zinc-800" />
                  </td>
                  <td className="py-4 pr-4">
                    <div className="h-3.5 w-1/2 animate-pulse rounded bg-zinc-800/80" />
                    <div className="mt-2 h-3 w-1/3 animate-pulse rounded bg-zinc-800/60" />
                  </td>
                  <td colSpan={5}>
                    <div className="h-3 w-3/4 animate-pulse rounded bg-zinc-800/60" />
                  </td>
                </tr>
              ))
            ) : pageRows.length === 0 ? (
              <tr>
                <td colSpan={7} className="py-16">
                  <div className="flex flex-col items-center text-center">
                    {filtersActive ? (
                      <>
                        <p className="text-sm font-medium text-zinc-200">
                          No rooms match these filters
                        </p>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={clearFilters}
                          className="mt-3"
                        >
                          Clear filters
                        </Button>
                      </>
                    ) : (
                      <>
                        <div className="flex h-20 w-20 items-center justify-center rounded-full bg-zinc-900 ring-1 ring-zinc-800">
                          <Users
                            className="h-9 w-9 text-zinc-600"
                            aria-hidden="true"
                          />
                        </div>
                        <p className="mt-4 text-sm font-medium text-zinc-200">
                          No rooms
                        </p>
                        <p className="mt-1 max-w-sm text-sm text-zinc-500">
                          Rooms people create in the app show up here.
                        </p>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ) : (
              pageRows.map((room) => (
                <RoomRow
                  key={room.id}
                  room={room}
                  selected={selected.has(room.id)}
                  onToggle={() => toggleRow(room.id)}
                  onCopy={copy}
                  onDelete={() => setPendingDelete([room])}
                />
              ))
            )}
          </tbody>
        </table>
      </div>

      {!isLoading && rows.length > 0 && (
        <div className="flex flex-wrap items-center justify-end gap-x-6 gap-y-2 py-3 text-sm text-zinc-400">
          <label className="flex items-center gap-2">
            Rows per page:
            <select
              value={rowsPerPage}
              onChange={(e) => {
                const n = Number(e.target.value);
                setRowsPerPage(n);
                setPage(0);
                try {
                  localStorage.setItem(ROWS_KEY, String(n));
                } catch {
                  /* not remembered */
                }
              }}
              className="h-8 rounded-md border border-zinc-700 bg-zinc-900 px-2 text-zinc-100 cursor-pointer"
            >
              {ROW_OPTIONS.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </label>
          <span className="tabular-nums">
            {currentPage * rowsPerPage + 1}–
            {Math.min(rows.length, (currentPage + 1) * rowsPerPage)} of{" "}
            {rows.length}
          </span>
          <div className="flex items-center gap-1">
            <IconButton
              label="Previous page"
              size="sm"
              disabled={currentPage === 0}
              onClick={() => setPage(currentPage - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </IconButton>
            <IconButton
              label="Next page"
              size="sm"
              disabled={currentPage >= pageCount - 1}
              onClick={() => setPage(currentPage + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </IconButton>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={!!pendingDelete}
        destructive
        title={
          pendingDelete && pendingDelete.length > 1
            ? `Permanently delete ${pendingDelete.length} rooms?`
            : "Permanently delete this room?"
        }
        description={
          <>
            {pendingDelete && pendingDelete.length === 1
              ? `"${pendingDelete[0].name}" will be deleted with its members, chat history and invitations.`
              : "These rooms will be deleted with their members, chat history and invitations."}
            {pendingConnected > 0 && (
              <span className="mt-2 block text-amber-300">
                {pendingConnected === 1
                  ? "1 person is"
                  : `${pendingConnected} people are`}{" "}
                in {pendingDelete && pendingDelete.length > 1 ? "them" : "it"}{" "}
                right now and will be sent back to the lobby.
              </span>
            )}
          </>
        }
        acknowledgement="I understand that deleting is permanent and can't be undone"
        confirmLabel="Delete forever"
        onConfirm={() => runDelete(pendingDelete ?? [])}
        onClose={() => setPendingDelete(null)}
      />
    </>
  );
}

interface RoomRowProps {
  room: AdminRoom;
  selected: boolean;
  onToggle: () => void;
  onCopy: (text: string, what: string) => void;
  onDelete: () => void;
}

function RoomRow({ room, selected, onToggle, onCopy, onDelete }: RoomRowProps) {
  const members = room.members?.length ?? 0;
  const live = room.activity;

  return (
    <tr
      className={cx(
        "group border-b border-zinc-800/70 transition-colors",
        selected ? "bg-emerald-500/[0.04]" : "hover:bg-zinc-900/70",
      )}
    >
      <td className="py-3 pl-1 align-top">
        <input
          type="checkbox"
          aria-label={`Select ${room.name}`}
          checked={selected}
          onChange={onToggle}
          className="mt-1 h-4 w-4 accent-emerald-500 cursor-pointer"
        />
      </td>
      <td className="py-3 pr-4 align-top">
        <div className="flex items-start gap-2">
          <div className="min-w-0 flex-1">
            <p className="line-clamp-1 text-sm font-medium text-zinc-100">
              {room.name}
            </p>
            <p className="mt-0.5 truncate font-mono text-xs text-zinc-500">
              {room.id}
            </p>
          </div>
          <div className="-my-1 flex shrink-0 items-center opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 [@media(hover:none)]:opacity-100">
            <IconButton
              label="Delete room"
              size="sm"
              onClick={onDelete}
              className="hover:text-rose-300"
            >
              <Trash2 className="h-4 w-4" />
            </IconButton>
            <Menu
              label={`Options for ${room.name}`}
              items={[
                {
                  label: "Copy room ID",
                  icon: Copy,
                  onSelect: () => onCopy(room.id, "Room ID"),
                },
                {
                  label: "Copy owner ID",
                  icon: Copy,
                  onSelect: () => onCopy(room.owner_id, "Owner ID"),
                },
                {
                  label: "Delete forever",
                  icon: Trash2,
                  destructive: true,
                  separated: true,
                  onSelect: onDelete,
                },
              ]}
              trigger={(props) => (
                <IconButton {...props} label="Options" size="sm">
                  <EllipsisVertical className="h-4 w-4" />
                </IconButton>
              )}
            />
          </div>
        </div>
      </td>
      <td className="py-3 pr-4 align-top">
        <span
          className={cx(
            "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs",
            room.is_private
              ? "border-zinc-700 text-zinc-300"
              : "border-sky-500/30 text-sky-300",
          )}
        >
          {room.is_private ? (
            <Lock className="h-3 w-3" aria-hidden="true" />
          ) : (
            <Globe className="h-3 w-3" aria-hidden="true" />
          )}
          {room.is_private ? "Private" : "Public"}
        </span>
      </td>
      <td className="py-3 pr-4 align-top">
        <span className="block truncate text-sm text-zinc-300">
          {ownerName(room)}
        </span>
      </td>
      <td className="py-3 pr-4 align-top text-sm tabular-nums text-zinc-300">
        {members}
      </td>
      <td className="py-3 pr-4 align-top">
        {live ? (
          <>
            <span className="inline-flex items-center gap-1.5 text-sm text-emerald-300">
              <span
                className="h-2 w-2 rounded-full bg-emerald-400"
                aria-hidden="true"
              />
              {live.viewers} watching
            </span>
            <span className="block text-xs text-zinc-500">
              {live.kind === "live" ? "Live stream" : "Video"} ·{" "}
              {live.is_playing ? "playing" : "paused"}
            </span>
          </>
        ) : (
          <span className="text-sm text-zinc-500">Empty</span>
        )}
      </td>
      <td className="py-3 pr-4 align-top">
        <span className="block text-sm text-zinc-300">
          {formatDate(room.created_at)}
        </span>
        {/* Past a month formatRelative gives the date again. */}
        {formatRelative(room.created_at) !== formatDate(room.created_at) && (
          <span className="text-xs text-zinc-500">
            {formatRelative(room.created_at)}
          </span>
        )}
      </td>
    </tr>
  );
}

import {
  Megaphone,
  Sparkles,
  TrendingUp,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import type { BadgeProps } from "@/components/ui/badge";
import type { CommunityCategory } from "@/types/community";

interface CategoryMeta {
  label: string;
  /** What the viewer badges it as, for the preview */
  viewerLabel: string;
  icon: LucideIcon;
  badge: NonNullable<BadgeProps["variant"]>;
}

export const CATEGORIES: Record<CommunityCategory, CategoryMeta> = {
  feature: {
    label: "Feature",
    viewerLabel: "New feature",
    icon: Sparkles,
    badge: "purple",
  },
  improvement: {
    label: "Improvement",
    viewerLabel: "Improvement",
    icon: TrendingUp,
    badge: "success",
  },
  fix: { label: "Fix", viewerLabel: "Fix", icon: Wrench, badge: "warning" },
  announcement: {
    label: "Announcement",
    viewerLabel: "Announcement",
    icon: Megaphone,
    badge: "default",
  },
};

export const CATEGORY_ORDER: CommunityCategory[] = [
  "feature",
  "improvement",
  "fix",
  "announcement",
];

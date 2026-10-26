import {
  Megaphone,
  Sparkles,
  TrendingUp,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import type { BadgeVariant } from "../common/Badge";
import type { CommunityCategory } from "../../types/community";

interface CategoryMeta {
  /** Singular, for a post's badge */
  label: string;
  /** Plural, for the filter tabs */
  plural: string;
  icon: LucideIcon;
  badge: BadgeVariant;
}

export const CATEGORIES: Record<CommunityCategory, CategoryMeta> = {
  feature: {
    label: "New feature",
    plural: "Features",
    icon: Sparkles,
    badge: "accent",
  },
  improvement: {
    label: "Improvement",
    plural: "Improvements",
    icon: TrendingUp,
    badge: "success",
  },
  fix: { label: "Fix", plural: "Fixes", icon: Wrench, badge: "warning" },
  announcement: {
    label: "Announcement",
    plural: "Announcements",
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

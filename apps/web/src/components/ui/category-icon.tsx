import {
  Banknote,
  Briefcase,
  Bus,
  Car,
  Coins,
  CreditCard,
  Flag,
  Gamepad2,
  GraduationCap,
  HandHeart,
  HeartPulse,
  House,
  Landmark,
  Laptop,
  type LucideIcon,
  PawPrint,
  PiggyBank,
  Plane,
  Plug,
  Receipt,
  Repeat,
  Shield,
  ShoppingCart,
  Tag,
  TrendingUp,
  Utensils,
  Wallet,
  Wifi,
} from "lucide-react";
import { cn } from "../../lib/cn";

const ICONS: Record<string, LucideIcon> = {
  banknote: Banknote,
  briefcase: Briefcase,
  bus: Bus,
  car: Car,
  coins: Coins,
  "credit-card": CreditCard,
  flag: Flag,
  "gamepad-2": Gamepad2,
  "graduation-cap": GraduationCap,
  "hand-heart": HandHeart,
  "heart-pulse": HeartPulse,
  house: House,
  landmark: Landmark,
  laptop: Laptop,
  "paw-print": PawPrint,
  "piggy-bank": PiggyBank,
  plane: Plane,
  plug: Plug,
  receipt: Receipt,
  repeat: Repeat,
  shield: Shield,
  "shopping-cart": ShoppingCart,
  tag: Tag,
  "trending-up": TrendingUp,
  utensils: Utensils,
  wallet: Wallet,
  wifi: Wifi,
};

export const CATEGORY_ICON_NAMES = Object.keys(ICONS);

export function iconByName(name: string | null | undefined): LucideIcon {
  return (name && ICONS[name]) || Tag;
}

export function formatCategoryLabel(
  category: { name: string; parentId?: string | null },
  all?: { id: string; name: string }[],
): string {
  if (!category.parentId || !all) return category.name;
  const parent = all.find((c) => c.id === category.parentId);
  return parent ? `${parent.name} › ${category.name}` : category.name;
}

/** Chip circular com o ícone da categoria tingido pela cor dela. */
export function CategoryIcon({
  icon,
  color,
  size = 36,
  className,
}: {
  icon: string | null | undefined;
  color: string | null | undefined;
  size?: number;
  className?: string;
}) {
  const Icon = iconByName(icon);
  const tint = color ?? "#71717a";
  return (
    <span
      className={cn("flex shrink-0 items-center justify-center rounded-full", className)}
      style={{ width: size, height: size, backgroundColor: `${tint}1f`, color: tint }}
      aria-hidden
    >
      <Icon size={Math.round(size * 0.5)} strokeWidth={2} />
    </span>
  );
}

"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  FileText,
  LayoutDashboard,
  MessageSquare,
  Package,
  ReceiptText,
  Tag,
  Users,
} from "lucide-react";

import {
  Sidebar as SidebarPrimitive,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
} from "@/components/ui/sidebar";

type NavigationItem = {
  label: string;
  href: string;
  icon: React.ComponentType<{ className?: string }>;
};

const navigationGroups: { label: string; items: NavigationItem[] }[] = [
  {
    label: "Overview",
    items: [{ label: "Dashboard", href: "/", icon: LayoutDashboard }],
  },
  {
    label: "Operasional",
    items: [
      { label: "Pesanan", href: "/orders", icon: ReceiptText },
      { label: "Produk", href: "/products", icon: Package },
      { label: "Kategori", href: "/categories", icon: Tag },
    ],
  },
  {
    label: "Pelanggan",
    items: [
      { label: "Pelanggan", href: "/customers", icon: Users },
      { label: "Inbox", href: "/inbox", icon: MessageSquare },
    ],
  },
  {
    label: "Sistem",
    items: [
      { label: "Promo", href: "/promos", icon: Tag },
      { label: "Audit log", href: "/audit-logs", icon: FileText },
    ],
  },
];

function isCurrentPath(pathname: string, href: string) {
  return href === "/" ? pathname === "/" : pathname.startsWith(href);
}

export function Sidebar() {
  const pathname = usePathname();

  return (
    <SidebarPrimitive collapsible="icon" className="border-sidebar-border">
      <SidebarHeader className="h-14 justify-center px-3">
        <Link
          href="/"
          className="flex min-w-0 items-center gap-2.5 rounded-md px-1 py-1 text-sm font-semibold tracking-tight text-sidebar-foreground outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
        >
          <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-slate-900 text-xs font-bold text-white">
            A
          </span>
          <span className="truncate group-data-[collapsible=icon]:hidden">
            Admin Platform
          </span>
        </Link>
      </SidebarHeader>

      <SidebarSeparator />

      <SidebarContent className="py-2">
        {navigationGroups.map((group) => (
          <SidebarGroup key={group.label} className="px-2 py-1">
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((item) => {
                  const active = isCurrentPath(pathname, item.href);
                  const Icon = item.icon;

                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton
                        render={<Link href={item.href} />}
                        isActive={active}
                        tooltip={item.label}
                        className="h-9 gap-2.5 px-2.5 text-sm"
                      >
                        <Icon className="size-4" />
                        <span>{item.label}</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="px-3 py-3">
        <p className="px-1 text-xs text-sidebar-foreground/60 group-data-[collapsible=icon]:hidden">
          Workspace admin
        </p>
      </SidebarFooter>
    </SidebarPrimitive>
  );
}
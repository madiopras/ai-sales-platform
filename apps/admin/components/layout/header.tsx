"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, LogOut } from "lucide-react";

import { Button } from "@/components/ui/button";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { useAuth } from "@/lib/auth-context";

function initials(name?: string) {
  if (!name) return "A";

  return name
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0])
    .join("")
    .toUpperCase();
}

export function Header() {
  const { user, logout } = useAuth();
  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false);
  const accountMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function closeOnOutsideClick(event: MouseEvent) {
      if (
        accountMenuRef.current &&
        !accountMenuRef.current.contains(event.target as Node)
      ) {
        setIsAccountMenuOpen(false);
      }
    }

    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape") setIsAccountMenuOpen(false);
    }

    document.addEventListener("mousedown", closeOnOutsideClick);
    document.addEventListener("keydown", closeOnEscape);

    return () => {
      document.removeEventListener("mousedown", closeOnOutsideClick);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, []);

  return (
    <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center justify-between border-b border-slate-200 bg-white px-4 sm:px-6">
      <div className="flex min-w-0 items-center gap-3">
        <SidebarTrigger className="-ml-1 size-8 text-slate-600 hover:bg-slate-100 hover:text-slate-950" />
        <span className="hidden h-4 w-px bg-slate-200 sm:block" />
        <p className="hidden truncate text-sm font-medium text-slate-700 sm:block">
          Workspace
        </p>
      </div>

      <div ref={accountMenuRef} className="relative">
        <button
          type="button"
          onClick={() => setIsAccountMenuOpen((open) => !open)}
          aria-expanded={isAccountMenuOpen}
          aria-haspopup="menu"
          className="flex h-9 items-center gap-2 rounded-md px-1.5 text-left outline-none transition-colors hover:bg-slate-100 focus-visible:ring-2 focus-visible:ring-slate-900"
        >
          <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-slate-900 text-[11px] font-semibold text-white">
            {initials(user?.name)}
          </span>
          <span className="hidden min-w-0 sm:block">
            <span className="block max-w-36 truncate text-sm font-medium leading-4 text-slate-800">
              {user?.name ?? "Admin"}
            </span>
            <span className="block max-w-36 truncate text-xs leading-4 text-slate-600">
              {user?.role ?? "Administrator"}
            </span>
          </span>
          <ChevronDown
            className={`hidden size-3.5 text-slate-500 transition-transform sm:block ${
              isAccountMenuOpen ? "rotate-180" : ""
            }`}
          />
        </button>

        {isAccountMenuOpen && (
          <div
            role="menu"
            aria-label="Menu akun"
            className="absolute right-0 top-[calc(100%+0.5rem)] z-30 w-64 rounded-lg border border-slate-200 bg-white p-1 shadow-lg shadow-slate-950/10"
          >
            <div className="px-3 py-2.5">
              <p className="truncate text-sm font-medium text-slate-900">
                {user?.name ?? "Admin"}
              </p>
              <p className="truncate text-xs text-slate-600">
                {user?.email ?? "Memuat akun..."}
              </p>
            </div>
            <div className="my-1 h-px bg-slate-100" />
            <Button
              variant="ghost"
              onClick={logout}
              className="h-9 w-full justify-start gap-2 px-3 text-sm text-slate-700 hover:bg-slate-100 hover:text-slate-950"
            >
              <LogOut className="size-4" />
              Keluar
            </Button>
          </div>
        )}
      </div>
    </header>
  );
}
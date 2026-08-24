import { Link } from "@tanstack/react-router";
import { Monitor, Moon, Star, Sun } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type PageKey = "icons" | "usage";
type ThemeMode = "system" | "light" | "dark";

const APP_NAME = "GMWallet Assets";
const REPOSITORY_URL = "https://github.com/GMWalletApp/assets";
const THEME_STORAGE_KEY = "gmwallet-assets-theme";

export function AppHeader({ activePage }: { activePage: PageKey }) {
  const [theme, setTheme] = useState<ThemeMode | null>(null);
  const activeTheme = theme ?? "system";

  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
      setTheme(stored === "light" || stored === "dark" ? stored : "system");
    } catch {
      setTheme("system");
    }
  }, []);

  useEffect(() => {
    if (!theme) return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const applyTheme = () => {
      const dark = theme === "dark" || (theme === "system" && media.matches);
      document.documentElement.classList.toggle("dark", dark);
      for (const meta of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
        meta.content = dark ? "#080a0f" : "#f7f8fa";
      }
    };
    applyTheme();
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, theme);
    } catch {
      // Theme still works when storage is unavailable.
    }
    if (theme !== "system") {
      return;
    }
    media.addEventListener("change", applyTheme);
    return () => media.removeEventListener("change", applyTheme);
  }, [theme]);

  return (
    <header className="sticky top-0 z-40 border-b bg-background/92 backdrop-blur-xl supports-[backdrop-filter]:bg-background/80">
      <div className="mx-auto flex min-h-16 max-w-[1536px] items-center gap-4 px-4 sm:px-6 lg:px-8">
        <Link className="flex shrink-0 items-center gap-2.5" to="/">
          <img
            alt="GMWallet"
            className="size-8 rounded-lg border bg-card object-cover shadow-xs"
            src={`${import.meta.env.BASE_URL}gmwallet-logo.jpg`}
          />
          <span className="hidden font-semibold tracking-tight sm:inline">{APP_NAME}</span>
        </Link>

        <nav aria-label="主导航" className="flex items-center gap-1">
          <NavLink active={activePage === "icons"} to="/">
            图标
          </NavLink>
          <NavLink active={activePage === "usage"} to="/usage">
            使用说明
          </NavLink>
        </nav>

        <div className="ml-auto flex items-center gap-1">
          <div className="hidden rounded-lg border bg-muted/60 p-0.5 sm:flex">
            <ThemeButton
              label="跟随系统"
              active={activeTheme === "system"}
              onClick={() => setTheme("system")}
            >
              <Monitor />
            </ThemeButton>
            <ThemeButton
              label="浅色"
              active={activeTheme === "light"}
              onClick={() => setTheme("light")}
            >
              <Sun />
            </ThemeButton>
            <ThemeButton
              label="深色"
              active={activeTheme === "dark"}
              onClick={() => setTheme("dark")}
            >
              <Moon />
            </ThemeButton>
          </div>
          <Button
            aria-label={`主题：${themeLabel(activeTheme)}`}
            className="sm:hidden"
            size="icon-sm"
            variant="outline"
            onClick={() => setTheme(nextTheme(activeTheme))}
          >
            {activeTheme === "system" ? <Monitor /> : activeTheme === "light" ? <Sun /> : <Moon />}
          </Button>
          <Button asChild size="sm" variant="outline">
            <a href={REPOSITORY_URL} rel="noreferrer" target="_blank">
              <Star />
              <span className="hidden sm:inline">GitHub</span>
            </a>
          </Button>
        </div>
      </div>
    </header>
  );
}

export function AppFooter({ meta }: { meta?: ReactNode }) {
  return (
    <footer className="mt-auto border-t bg-card/50">
      <div className="mx-auto flex max-w-[1536px] flex-col gap-3 px-4 py-6 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8">
        <span>© 2026 {APP_NAME}</span>
        {meta}
      </div>
    </footer>
  );
}

function NavLink({
  active,
  children,
  to,
}: {
  active: boolean;
  children: ReactNode;
  to: "/" | "/usage";
}) {
  return (
    <Link
      className={cn(
        "rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        active && "bg-muted text-foreground",
      )}
      to={to}
    >
      {children}
    </Link>
  );
}

function ThemeButton({
  active,
  children,
  label,
  onClick,
}: {
  active: boolean;
  children: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <Button
      aria-label={label}
      aria-pressed={active}
      className={cn(active && "bg-background text-foreground shadow-xs")}
      size="icon-sm"
      variant="ghost"
      onClick={onClick}
    >
      {children}
    </Button>
  );
}

function nextTheme(theme: ThemeMode): ThemeMode {
  if (theme === "system") return "light";
  if (theme === "light") return "dark";
  return "system";
}

function themeLabel(theme: ThemeMode): string {
  if (theme === "system") return "跟随系统";
  return theme === "light" ? "浅色" : "深色";
}

import { Link } from "@tanstack/react-router";
import { Monitor, Moon, Sun } from "lucide";
import { MorphIcon } from "morphicons/react";
import { type ReactNode, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type PageKey = "icons" | "usage";
type ThemeMode = "system" | "light" | "dark";

const APP_NAME = "GMWallet Assets";
const REPOSITORY_URL = "https://github.com/GMWalletApp/assets";
const THEME_STORAGE_KEY = "gmwallet-assets-theme";
const ENGLISH_LANGUAGE_ICON = "M5 19 10.8 5.5a1.3 1.3 0 0 1 2.4 0L19 19M8 14h8";
const CHINESE_LANGUAGE_ICON = "M12 3v3M5 8h14M8 11c1.4 4.2 4.4 7.2 9 9M16 11c-1.4 4.2-4.4 7.2-9 9";

export function AppHeader({ activePage }: { activePage: PageKey }) {
  const { i18n, t } = useTranslation();
  const isChinese = i18n.resolvedLanguage === "zh-CN";
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
    <header className="sticky top-0 z-40 border-b bg-background/92 backdrop-blur-xl supports-backdrop-filter:bg-background/80">
      <div className="mx-auto flex min-h-16 max-w-384 items-center gap-1 px-3 sm:gap-4 sm:px-6 lg:px-8">
        <Link className="flex shrink-0 items-center gap-2.5" to="/">
          <img
            alt="GMWallet"
            className="size-8 rounded-lg border bg-card object-cover shadow-xs"
            src={`${import.meta.env.BASE_URL}gmwallet-logo.jpg`}
          />
          <span className="hidden font-semibold tracking-tight sm:inline">{APP_NAME}</span>
        </Link>

        <nav aria-label={t("nav.main")} className="flex items-center gap-1">
          <NavLink active={activePage === "icons"} to="/">
            {t("nav.icons")}
          </NavLink>
          <NavLink active={activePage === "usage"} mobileLabel={t("nav.usageMobile")} to="/usage">
            {t("nav.usage")}
          </NavLink>
        </nav>

        <div className="ml-auto flex items-center gap-1">
          <Button
            aria-label={`${t("theme.label")}: ${themeLabel(activeTheme, t)}`}
            size="icon"
            title={`${t("theme.label")}: ${themeLabel(activeTheme, t)}`}
            variant="outline"
            onClick={() => setTheme(nextTheme(activeTheme))}
          >
            <MorphIcon
              className="size-4.5"
              data-icon="inline-start"
              icon={themeIcon(activeTheme)}
              reducedMotion="user"
              spring="snappy"
            />
          </Button>
          <Button
            aria-label={t("actions.switchLanguage")}
            size="icon"
            title={t("actions.switchLanguage")}
            variant="outline"
            onClick={() => void i18n.changeLanguage(isChinese ? "en-US" : "zh-CN")}
          >
            <MorphIcon
              className="size-4.5"
              data-icon="inline-start"
              icon={isChinese ? ENGLISH_LANGUAGE_ICON : CHINESE_LANGUAGE_ICON}
              reducedMotion="user"
              spring="snappy"
            />
          </Button>
          <Button asChild size="icon" variant="outline">
            <a
              aria-label={t("actions.github")}
              href={REPOSITORY_URL}
              rel="noreferrer"
              target="_blank"
              title={t("actions.github")}
            >
              <GitHubIcon />
            </a>
          </Button>
        </div>
      </div>
    </header>
  );
}

function GitHubIcon() {
  return (
    <svg aria-hidden="true" className="size-4.5" fill="currentColor" viewBox="0 0 24 24">
      <path d="M12 .7a11.5 11.5 0 0 0-3.6 22.4c.6.1.8-.3.8-.6v-2.2c-3.4.7-4.1-1.4-4.1-1.4-.5-1.4-1.3-1.8-1.3-1.8-1.1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1.1 1.8 2.8 1.3 3.5 1 .1-.8.4-1.3.8-1.6-2.7-.3-5.5-1.3-5.5-5.7 0-1.3.5-2.3 1.2-3.1-.1-.3-.5-1.5.1-3.1 0 0 1-.3 3.2 1.2a11 11 0 0 1 5.8 0C14.9 4.8 16 5.1 16 5.1c.6 1.6.2 2.8.1 3.1.8.8 1.2 1.8 1.2 3.1 0 4.4-2.8 5.4-5.5 5.7.4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6A11.5 11.5 0 0 0 12 .7Z" />
    </svg>
  );
}

export function AppFooter({ meta }: { meta?: ReactNode }) {
  return (
    <footer className="mt-auto border-t bg-card/50">
      <div className="mx-auto flex max-w-384 flex-col gap-1.5 px-4 py-5 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between sm:gap-3 sm:px-6 sm:py-6 lg:px-8">
        <span>© 2026 {APP_NAME}</span>
        {meta}
      </div>
    </footer>
  );
}

function NavLink({
  active,
  children,
  mobileLabel,
  to,
}: {
  active: boolean;
  children: ReactNode;
  mobileLabel?: string;
  to: "/" | "/usage";
}) {
  return (
    <Link
      className={cn(
        "rounded-md px-2 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:px-3",
        active && "bg-muted text-foreground",
      )}
      to={to}
    >
      {mobileLabel ? <span className="sm:hidden">{mobileLabel}</span> : null}
      <span className={mobileLabel ? "hidden sm:inline" : undefined}>{children}</span>
    </Link>
  );
}

function themeIcon(theme: ThemeMode) {
  if (theme === "light") return Sun;
  if (theme === "dark") return Moon;
  return Monitor;
}

function nextTheme(theme: ThemeMode): ThemeMode {
  if (theme === "system") return "light";
  if (theme === "light") return "dark";
  return "system";
}

function themeLabel(theme: ThemeMode, t: ReturnType<typeof useTranslation>["t"]): string {
  if (theme === "system") return t("theme.system");
  return theme === "light" ? t("theme.light") : t("theme.dark");
}

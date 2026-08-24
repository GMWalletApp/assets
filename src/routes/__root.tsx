import { createRootRoute, HeadContent, Outlet, Scripts } from "@tanstack/react-router";
import type { ReactNode } from "react";
import "../styles.css";

const themeScript = `(() => { try { const value = localStorage.getItem("gmwallet-assets-theme") || "system"; const dark = value === "dark" || (value === "system" && matchMedia("(prefers-color-scheme: dark)").matches); document.documentElement.classList.toggle("dark", dark); } catch {} })();`;

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1" },
      { name: "theme-color", content: "#f7f8fa", media: "(prefers-color-scheme: light)" },
      { name: "theme-color", content: "#080a0f", media: "(prefers-color-scheme: dark)" },
      {
        name: "description",
        content: "Browse GMWallet token, network, exchange, wallet, and dapp assets.",
      },
      { title: "GMWallet Asset Atlas" },
    ],
    links: [
      { rel: "icon", href: `${import.meta.env.BASE_URL}favicon.ico`, sizes: "any" },
      { rel: "apple-touch-icon", href: `${import.meta.env.BASE_URL}logo192.png` },
      { rel: "manifest", href: `${import.meta.env.BASE_URL}manifest.json` },
    ],
  }),
  component: Root,
});

function Root() {
  return (
    <RootDocument>
      <Outlet />
    </RootDocument>
  );
}

function RootDocument({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        {/* biome-ignore lint/security/noDangerouslySetInnerHtml: static local theme bootstrap prevents a color flash */}
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  );
}

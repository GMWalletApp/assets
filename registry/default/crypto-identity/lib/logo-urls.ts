import { ASSETS_BRANCH, ASSETS_REPOSITORY, orderedCdnBaseUrls } from "./constants";
import { uniqueUrls } from "./normalize";

const JSDELIVR_HOSTS = new Set([
  "cdn.jsdmirror.com",
  "cdn.jsdelivr.net",
  "fastly.jsdelivr.net",
  "gcore.jsdelivr.net",
]);

/** Normalize supported catalog URLs to one copyable jsDelivr URL. */
export function canonicalLogoUrl(logoUrl: string): string {
  try {
    const parsed = new URL(logoUrl);
    if (parsed.hostname === "assets-cdn.trustwallet.com") {
      return `https://cdn.jsdelivr.net/gh/trustwallet/assets@master${parsed.pathname}`;
    }
    if (JSDELIVR_HOSTS.has(parsed.hostname)) {
      return `https://cdn.jsdelivr.net${parsed.pathname}`;
    }
    if (parsed.hostname !== "raw.githubusercontent.com") {
      return logoUrl;
    }
    const [owner, repository, branch, ...path] = parsed.pathname.split("/").filter(Boolean);
    return owner && repository && branch && path.length > 0
      ? `https://cdn.jsdelivr.net/gh/${owner}/${repository}@${branch}/${path.join("/")}`
      : logoUrl;
  } catch {
    return logoUrl;
  }
}

/** Convert a catalog logoURI into ordered CDN candidates while preserving the original URL. */
export function resolveLogoUrls(logoUrl: string, preferredBaseUrl?: string): string[] {
  try {
    const parsed = new URL(logoUrl);
    if (parsed.hostname === "assets-cdn.trustwallet.com") {
      return uniqueUrls([
        `https://cdn.jsdmirror.com/gh/trustwallet/assets@master${parsed.pathname}`,
        canonicalLogoUrl(logoUrl),
        logoUrl,
      ]);
    }
    if (JSDELIVR_HOSTS.has(parsed.hostname)) {
      return uniqueUrls([
        `https://cdn.jsdmirror.com${parsed.pathname}`,
        canonicalLogoUrl(logoUrl),
        logoUrl,
      ]);
    }
    if (parsed.hostname !== "raw.githubusercontent.com") {
      return [logoUrl];
    }

    const [owner, repository, branch, ...path] = parsed.pathname.split("/").filter(Boolean);
    if (!owner || !repository || !branch || path.length === 0) {
      return [logoUrl];
    }
    if (`${owner}/${repository}` === ASSETS_REPOSITORY && branch === ASSETS_BRANCH) {
      return uniqueUrls([
        ...orderedCdnBaseUrls(preferredBaseUrl).map((baseUrl) => `${baseUrl}/${path.join("/")}`),
        logoUrl,
      ]);
    }
    return uniqueUrls([
      `https://cdn.jsdmirror.com/gh/${owner}/${repository}@${branch}/${path.join("/")}`,
      canonicalLogoUrl(logoUrl),
      logoUrl,
    ]);
  } catch {
    return [logoUrl];
  }
}

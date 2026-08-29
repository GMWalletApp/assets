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
  return (
    replaceAssetBaseUrls(logoUrl, [
      `https://cdn.jsdelivr.net/gh/${ASSETS_REPOSITORY}@${ASSETS_BRANCH}`,
    ])[0] ?? logoUrl
  );
}

/** Convert a catalog logoURI path into ordered URLs for the configured assets repository. */
export function resolveLogoUrls(logoUrl: string, preferredBaseUrl?: string): string[] {
  const urls = replaceAssetBaseUrls(logoUrl, orderedCdnBaseUrls(preferredBaseUrl));
  return urls.length > 0 ? uniqueUrls(urls) : [logoUrl];
}

function replaceAssetBaseUrls(logoUrl: string, baseUrls: readonly string[]): string[] {
  let source: URL;
  try {
    source = new URL(logoUrl);
  } catch {
    return [];
  }
  const parts = source.pathname.split("/").filter(Boolean);
  let path: string[];
  if (source.hostname === "assets-cdn.trustwallet.com") {
    path = parts;
  } else if (source.hostname === "raw.githubusercontent.com") {
    path = parts.slice(3);
  } else if (JSDELIVR_HOSTS.has(source.hostname) && parts[0] === "gh") {
    path = parts.slice(3);
  } else {
    return [];
  }
  if (path.length === 0) {
    return [];
  }
  const suffix = path.join("/");
  return baseUrls.map((baseUrl) => `${baseUrl.replace(/\/$/, "")}/${suffix}`);
}

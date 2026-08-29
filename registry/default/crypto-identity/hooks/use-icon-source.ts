"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { normalize, normalizeAssetName, normalizeNetworkSlug } from "../lib/normalize";
import { resolveIconUrls } from "../lib/resolve-icon-urls";
import type { CryptoIdentityIcon } from "../lib/types";

export function useIconSource(icon?: CryptoIdentityIcon, preferredBaseUrl?: string) {
  const key = `${stableIconKey(icon)}:${preferredBaseUrl ?? ""}`;
  const iconRef = useRef(icon);
  iconRef.current = icon;
  const [result, setResult] = useState<{ key: string; sources: string[] }>({
    key: "",
    sources: [],
  });
  const [index, setIndex] = useState(0);
  const sources = result.key === key ? result.sources : [];

  useEffect(() => {
    const currentIcon = iconRef.current;
    setIndex(0);
    let cancelled = false;

    if (!currentIcon) {
      return;
    }

    resolveIconUrls(currentIcon, preferredBaseUrl).then((urls) => {
      if (cancelled) {
        return;
      }
      setResult({ key, sources: urls });
    });

    return () => {
      cancelled = true;
    };
  }, [key, preferredBaseUrl]);

  const handleError = useCallback(() => {
    setIndex((value) => Math.min(value + 1, sources.length));
  }, [sources.length]);

  return {
    src: sources[index],
    isResolving: Boolean(icon) && result.key !== key,
    handleError,
  };
}

function stableIconKey(icon?: CryptoIdentityIcon): string {
  if (!icon) {
    return "";
  }
  return [
    icon.type,
    normalizeAssetName(icon.type, icon.name),
    icon.type === "token" ? normalizeNetworkSlug(icon.network) : "",
    icon.type === "token" ? normalize(icon.contractAddress) : "",
  ].join(":");
}

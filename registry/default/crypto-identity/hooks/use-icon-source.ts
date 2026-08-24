"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { resolveIconUrls } from "../lib/resolve-icon-urls";
import type { AssetQuery, CryptoIdentityIcon } from "../lib/types";

export function useIconSource(icon?: CryptoIdentityIcon, preferredBaseUrl?: string) {
  const key = `${stableIconKey(icon)}:${preferredBaseUrl ?? ""}`;
  const activeKeyRef = useRef(key);
  const iconRef = useRef(icon);
  iconRef.current = icon;
  const catalogRequestedRef = useRef(false);
  const [sources, setSources] = useState<string[]>([]);
  const [index, setIndex] = useState(0);
  const [isResolving, setIsResolving] = useState(Boolean(icon));

  useEffect(() => {
    const currentIcon = iconRef.current;
    activeKeyRef.current = key;
    setSources([]);
    setIndex(0);
    setIsResolving(Boolean(currentIcon));
    catalogRequestedRef.current = false;
    let cancelled = false;

    if (!currentIcon) {
      return;
    }

    resolveSources(currentIcon, preferredBaseUrl).then((urls) => {
      if (cancelled) {
        return;
      }
      setSources(urls);
      if (urls.length === 0 && currentIcon.type === "token") {
        catalogRequestedRef.current = true;
        resolveSources({ ...currentIcon, includeCatalog: true }, preferredBaseUrl).then(
          (catalogUrls) => {
            if (!cancelled) {
              setSources(catalogUrls);
              setIsResolving(false);
            }
          },
        );
        return;
      }
      setIsResolving(false);
    });

    return () => {
      cancelled = true;
    };
  }, [key, preferredBaseUrl]);

  const handleError = useCallback(() => {
    if (index + 1 < sources.length) {
      setIndex((value) => value + 1);
      return;
    }

    const currentIcon = iconRef.current;
    if (!currentIcon) {
      return;
    }
    if (catalogRequestedRef.current) {
      setIndex(sources.length);
      return;
    }

    if (currentIcon.type === "network" || currentIcon.type === "dapp") {
      setIndex(sources.length);
      return;
    }

    catalogRequestedRef.current = true;
    setIsResolving(true);
    const requestedKey = key;
    resolveSources({ ...currentIcon, includeCatalog: true }, preferredBaseUrl).then((urls) => {
      if (activeKeyRef.current !== requestedKey) {
        return;
      }
      const fallbackUrls = urls.filter((url) => !sources.includes(url));
      setSources(fallbackUrls);
      setIndex(0);
      setIsResolving(false);
    });
  }, [index, key, preferredBaseUrl, sources]);

  return { src: sources[index], isResolving, handleError };
}

function resolveSources(icon: AssetQuery, preferredBaseUrl?: string) {
  return preferredBaseUrl ? resolveIconUrls(icon, preferredBaseUrl) : resolveIconUrls(icon);
}

function stableIconKey(icon?: CryptoIdentityIcon): string {
  if (!icon) {
    return "";
  }
  return [
    icon.type,
    icon.name,
    icon.type === "token" ? icon.network : "",
    icon.type === "token" ? (icon.contractAddress ?? "") : "",
  ].join(":");
}

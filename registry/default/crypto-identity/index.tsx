"use client";

import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentPropsWithRef, ReactNode } from "react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useIconSource } from "./hooks/use-icon-source";
import { useImageBackground } from "./hooks/use-image-background";
import type { CryptoIdentityIcon } from "./lib/types";

export type { CryptoIdentityIcon } from "./lib/types";

const adaptiveBackground =
  "[background:var(--crypto-identity-light-surface)] dark:[background:var(--crypto-identity-dark-surface)]";

export const cryptoIdentityVariants = cva("inline-flex", {
  variants: {
    variant: {
      avatar: "relative size-8 shrink-0",
      badge:
        "min-w-0 items-center gap-1.5 rounded-full bg-secondary px-2 py-1 text-secondary-foreground",
      label: "min-w-0 items-center gap-2",
    },
  },
  defaultVariants: {
    variant: "avatar",
  },
});

const avatarVariants = cva("", {
  variants: {
    variant: {
      avatar: "size-full",
      badge: "size-4",
      label: "size-5",
    },
  },
  defaultVariants: {
    variant: "avatar",
  },
});

export type CryptoIdentityVariant = NonNullable<
  VariantProps<typeof cryptoIdentityVariants>["variant"]
>;

export interface CryptoIdentityProps extends ComponentPropsWithRef<"span"> {
  icon: CryptoIdentityIcon;
  cornerIcon?: CryptoIdentityIcon | undefined;
  fallback?: ReactNode | undefined;
  cornerFallback?: ReactNode | undefined;
  preferredBaseUrl?: string | undefined;
  variant?: CryptoIdentityVariant;
}

export function CryptoIdentity({
  children,
  className,
  cornerFallback,
  cornerIcon,
  fallback,
  icon,
  preferredBaseUrl,
  variant = "avatar",
  ref,
  ...props
}: CryptoIdentityProps) {
  const image = useIconSource(icon, preferredBaseUrl);
  const corner = useIconSource(cornerIcon, preferredBaseUrl);
  const imageBackground = useImageBackground(image.src);
  const cornerBackground = useImageBackground(corner.src);
  const imageLoading = image.isResolving || Boolean(image.src && !imageBackground.isLoaded);
  const cornerLoading = corner.isResolving || Boolean(corner.src && !cornerBackground.isLoaded);

  return (
    <span
      ref={ref}
      data-slot="crypto-identity"
      className={cn(cryptoIdentityVariants({ variant }), className)}
      {...props}
    >
      <span
        data-slot="crypto-identity-avatar"
        className={cn("relative inline-flex shrink-0", avatarVariants({ variant }))}
      >
        <Avatar
          aria-busy={imageLoading || undefined}
          data-slot="crypto-identity-image"
          className={cn("size-full bg-muted", imageBackground.style && adaptiveBackground)}
          style={imageBackground.style}
        >
          {image.src ? (
            <AvatarImage
              alt={variant === "avatar" ? icon.name : ""}
              className="dark:[filter:var(--crypto-identity-dark-filter,none)]"
              crossOrigin={imageBackground.crossOrigin}
              src={image.src}
              onError={image.handleError}
              onLoad={imageBackground.handleLoad}
            />
          ) : null}
          <AvatarFallback>{fallback}</AvatarFallback>
          {imageLoading ? (
            <Skeleton
              aria-hidden="true"
              data-slot="crypto-identity-skeleton"
              className="pointer-events-none absolute inset-0 size-full rounded-full"
            />
          ) : null}
        </Avatar>
        {cornerIcon ? (
          <Avatar
            aria-busy={cornerLoading || undefined}
            data-slot="crypto-identity-corner"
            className={cn(
              "absolute -right-0.5 -bottom-0.5 size-[40%] min-h-3 min-w-3 bg-muted ring-2 ring-background",
              cornerBackground.style && adaptiveBackground,
            )}
            style={cornerBackground.style}
          >
            {corner.src ? (
              <AvatarImage
                alt=""
                className="dark:[filter:var(--crypto-identity-dark-filter,none)]"
                crossOrigin={cornerBackground.crossOrigin}
                src={corner.src}
                onError={corner.handleError}
                onLoad={cornerBackground.handleLoad}
              />
            ) : null}
            <AvatarFallback>{cornerFallback}</AvatarFallback>
            {cornerLoading ? (
              <Skeleton
                aria-hidden="true"
                data-slot="crypto-identity-corner-skeleton"
                className="pointer-events-none absolute inset-0 size-full rounded-full"
              />
            ) : null}
          </Avatar>
        ) : null}
      </span>
      {variant === "avatar" ? null : (
        <span data-slot="crypto-identity-label" className="truncate">
          {children}
        </span>
      )}
    </span>
  );
}

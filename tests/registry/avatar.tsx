// Lightweight shadcn Avatar test double.
import type { ComponentPropsWithRef, HTMLAttributes, ImgHTMLAttributes } from "react";

export function Avatar({ children, ref, ...props }: ComponentPropsWithRef<"span">) {
  return (
    <span ref={ref} {...props}>
      {children}
    </span>
  );
}

export function AvatarImage(props: ImgHTMLAttributes<HTMLImageElement>) {
  return <img alt="" {...props} />;
}

export function AvatarFallback(props: HTMLAttributes<HTMLSpanElement>) {
  return <span {...props} />;
}

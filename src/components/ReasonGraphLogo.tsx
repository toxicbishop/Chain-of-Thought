import { cn } from "@/lib/utils";

export function ReasonGraphLogo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 32 32"
      role="img"
      aria-label="ReasonGraph"
      className={cn("h-5 w-5", className)}
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M9 10.5L16 6L23 10.5V19.5L16 26L9 19.5V10.5Z"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <path
        d="M9.5 11L16 16M22.5 11L16 16M16 16V25"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
      />
      <circle cx="16" cy="6" r="2.4" fill="currentColor" />
      <circle cx="9" cy="10.5" r="2.4" fill="currentColor" />
      <circle cx="23" cy="10.5" r="2.4" fill="currentColor" />
      <circle cx="16" cy="16" r="3" fill="currentColor" />
      <circle cx="16" cy="26" r="2.4" fill="currentColor" />
    </svg>
  );
}

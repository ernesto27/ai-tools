// Shared building blocks so screens don't repeat long Tailwind class lists.
import type { ComponentProps, ReactNode } from "react";

export const cx = (...classes: (string | false | null | undefined)[]) => classes.filter(Boolean).join(" ");

// ---------- Text ----------

export function Title({ children }: { children: ReactNode }) {
  return <h1 className="text-3xl font-semibold tracking-[-0.015em]">{children}</h1>;
}

export function Crumb({ children }: { children: ReactNode }) {
  return <span className="text-md text-muted">{children}</span>;
}

export function SectionHeading({
  as: Tag = "h2",
  plain,
  className,
  ...props
}: ComponentProps<"h2"> & { as?: "h2" | "h3"; plain?: boolean }) {
  return <Tag className={cx("text-md font-medium text-muted", !plain && "pb-3 border-b border-line", className)} {...props} />;
}

// ---------- Buttons ----------

export function PrimaryButton({ size = "md", className, ...props }: ComponentProps<"button"> & { size?: "md" | "lg" }) {
  return (
    <button
      type="button"
      className={cx(
        "inline-flex items-center rounded-md bg-ink font-medium text-canvas hover:bg-ink-strong",
        size === "lg" ? "min-h-10 px-[18px]" : "min-h-9 px-3.5",
        className,
      )}
      {...props}
    />
  );
}

export function OutlineButton({ className, ...props }: ComponentProps<"button">) {
  return (
    <button
      type="button"
      className={cx("min-h-10 rounded-md border border-field-line px-3.5 text-md text-ink hover:border-muted", className)}
      {...props}
    />
  );
}

const TEXT_TONE = { ink: "text-ink", muted: "text-muted", danger: "text-bad" };

/** Borderless action, underlined on hover ("Open diff", "Remove worktree"). */
export function TextButton({ tone = "ink", className, ...props }: ComponentProps<"button"> & { tone?: keyof typeof TEXT_TONE }) {
  return <button type="button" className={cx("min-h-7 underline-offset-4 hover:underline", TEXT_TONE[tone], className)} {...props} />;
}

/** Always-underlined action that reads as a link ("Continue", "Resume", "Cancel"). */
export function LinkButton({ muted, className, ...props }: ComponentProps<"button"> & { muted?: boolean }) {
  return (
    <button
      type="button"
      className={cx("underline", muted ? "text-muted hover:text-ink" : "text-ink hover:text-ink-strong", className)}
      {...props}
    />
  );
}

// ---------- Form fields ----------

const FIELD = "rounded-md border border-field-line bg-field text-ink placeholder:text-faint focus:border-muted focus:outline-none";

export function Field({ label, hint, children }: { label: ReactNode; hint?: ReactNode; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5 text-sm text-muted">
      {label}
      {children}
      {hint && <span className="text-xs">{hint}</span>}
    </label>
  );
}

export function Input({ mono, large, className, ...props }: ComponentProps<"input"> & { mono?: boolean; large?: boolean }) {
  return (
    <input
      type="text"
      className={cx(FIELD, mono ? "font-mono text-md" : "text-lg", large ? "min-h-10 px-3" : "min-h-9.5 px-2.5", className)}
      {...props}
    />
  );
}

export function Select({ className, ...props }: ComponentProps<"select">) {
  return <select className={cx(FIELD, "min-h-9.5 px-2 text-lg", className)} {...props} />;
}

export function Textarea({ className, ...props }: ComponentProps<"textarea">) {
  return <textarea className={cx(FIELD, "w-full resize-y p-3.5 font-mono text-md leading-[1.65]", className)} {...props} />;
}

export function Checkbox({ children, tight, className, ...props }: ComponentProps<"input"> & { tight?: boolean }) {
  return (
    <label className={cx("flex min-h-9 cursor-pointer items-center", tight ? "gap-2" : "gap-2.5", className)}>
      <input type="checkbox" className="m-0 size-4 accent-ink" {...props} />
      {children}
    </label>
  );
}

// ---------- Tabs ----------

interface TabsProps<T extends string> {
  label: string;
  items: readonly (readonly [T, string])[];
  value: T;
  onChange: (value: T) => void;
  size?: "md" | "sm";
}

/** Underlined tab bar (New run modes, Run detail views). */
export function Tabs<T extends string>({ label, items, value, onChange, size = "md", bordered = true }: TabsProps<T> & { bordered?: boolean }) {
  return (
    <div role="tablist" aria-label={label} className={cx("flex", size === "sm" ? "gap-6" : "gap-7", bordered && "border-b border-line")}>
      {items.map(([key, text]) => (
        <button
          key={key}
          type="button"
          role="tab"
          aria-selected={value === key}
          onClick={() => onChange(key)}
          className={cx(
            "-mb-px min-h-10 border-b-2 border-transparent text-muted aria-selected:border-ink aria-selected:font-medium aria-selected:text-ink",
            size === "sm" ? "text-base" : "text-lg",
          )}
        >
          {text}
        </button>
      ))}
    </div>
  );
}

/** Small text switcher, underlined when selected (task source, settings source). */
export function TextTabs<T extends string>({ label, items, value, onChange, size = "md" }: TabsProps<T>) {
  return (
    <div role="tablist" aria-label={label} className={cx("flex", size === "sm" ? "gap-3 text-sm" : "gap-4 text-md")}>
      {items.map(([key, text]) => (
        <button
          key={key}
          type="button"
          role="tab"
          aria-selected={value === key}
          onClick={() => onChange(key)}
          className={cx(
            "text-muted underline-offset-4 aria-selected:text-ink aria-selected:underline",
            size === "sm" ? "min-h-7" : "min-h-8",
          )}
        >
          {text}
        </button>
      ))}
    </div>
  );
}

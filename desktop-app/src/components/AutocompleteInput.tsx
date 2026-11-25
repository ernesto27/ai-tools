import { useRef, type ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import { cx, Input } from "./ui";

type Props = ComponentProps<typeof Input> & {
  id: string;
  fieldLabel: string;
  onClear?: () => void;
};

export function AutocompleteInput({ id, fieldLabel, onClear, disabled, className, ...props }: Props) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const clearLabel = t("clearAutocompleteSelection", { field: fieldLabel });

  return (
    <div className="relative min-w-0 w-full">
      <Input
        {...props}
        ref={input}
        id={id}
        disabled={disabled}
        className={cx("min-w-0 w-full", !!onClear && "pe-9!", className)}
      />
      {onClear && (
        <button
          id={`${id}-clear`}
          type="button"
          aria-label={clearLabel}
          title={clearLabel}
          disabled={disabled}
          className="absolute inset-y-1 end-1 flex w-7 items-center justify-center rounded text-lg text-muted enabled:hover:bg-hover enabled:hover:text-ink focus-visible:outline-2 focus-visible:outline-info disabled:opacity-40"
          onClick={() => {
            input.current?.focus();
            onClear();
          }}
        >
          <span aria-hidden="true">×</span>
        </button>
      )}
    </div>
  );
}

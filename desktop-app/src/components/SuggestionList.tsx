import { cx } from "./ui";

interface Props {
  id: string;
  label: string;
  options: { value: string; label: string }[];
  activeIndex: number;
  selectedIndex: number;
  openAbove: boolean;
  message?: string;
  onSelect: (index: number) => void;
}

export function SuggestionList({ id, label, options, activeIndex, selectedIndex, openAbove, message, onSelect }: Props) {
  return (
    <div className={cx(
      "absolute left-0 z-20 w-full min-w-0 rounded-md border border-field-line bg-panel shadow-lg",
      openAbove ? "bottom-full mb-1" : "top-full mt-1",
    )}>
      <div id={id} role="listbox" aria-label={label} className="max-h-52 overflow-y-auto p-1">
        {options.map((option, index) => (
          <div
            key={option.value}
            id={`${id}-${index}`}
            role="option"
            aria-selected={index === selectedIndex}
            className={cx(
              "cursor-pointer rounded px-2.5 py-2 font-mono text-sm break-all text-ink hover:bg-selected",
              index === activeIndex && "bg-selected",
            )}
            onPointerDown={(event) => event.preventDefault()}
            onClick={(event) => { event.preventDefault(); onSelect(index); }}
          >
            {option.label}
          </div>
        ))}
      </div>
      {message && <p role="status" className="px-3 py-2 text-sm text-muted">{message}</p>}
    </div>
  );
}

import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import type { KeyboardEvent } from "react";
import { MODEL_CATALOG } from "../modelCatalog";
import { cx, Input } from "./ui";

interface Props {
  id: string;
  agent: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
}

export function ModelAutocomplete({ id, agent, value, onChange, placeholder, disabled = false }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [openAbove, setOpenAbove] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const models = Object.prototype.hasOwnProperty.call(MODEL_CATALOG, agent)
    ? MODEL_CATALOG[agent]
    : [];
  const query = value.trim().toLowerCase();
  const matches = models.filter((model) => model.toLowerCase().includes(query));
  const expanded = open && !disabled && models.length > 0;
  const activeModel = matches[activeIndex];
  const listId = `${id}-suggestions`;
  const activeId = expanded && activeModel ? `${listId}-${activeIndex}` : undefined;

  useEffect(() => {
    if (activeId) document.getElementById(activeId)?.scrollIntoView({ block: "nearest" });
  }, [activeId]);

  function showSuggestions(input: HTMLInputElement) {
    const bounds = input.getBoundingClientRect();
    const spaceBelow = window.innerHeight - bounds.bottom;
    setOpenAbove(spaceBelow < 208 && bounds.top > spaceBelow);
    setOpen(true);
  }

  function select(model: string) {
    if (disabled) return;
    onChange(model);
    setOpen(false);
    setActiveIndex(-1);
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.nativeEvent.isComposing || disabled) return;
    if (event.key === "Escape") {
      if (expanded) event.preventDefault();
      setOpen(false);
      setActiveIndex(-1);
      return;
    }
    if (event.key === "Tab") {
      setOpen(false);
      setActiveIndex(-1);
      return;
    }
    if ((event.key === "ArrowDown" || event.key === "ArrowUp") && matches.length) {
      event.preventDefault();
      showSuggestions(event.currentTarget);
      setActiveIndex((current) => {
        if (!expanded || current < 0 || current >= matches.length) {
          return event.key === "ArrowDown" ? 0 : matches.length - 1;
        }
        return (current + (event.key === "ArrowDown" ? 1 : -1) + matches.length) % matches.length;
      });
    } else if (event.key === "Enter" && expanded && activeModel) {
      event.preventDefault();
      select(activeModel);
    }
  }

  return (
    <div className="relative min-w-0 w-full">
      <Input
        id={id}
        className="min-w-0 w-full"
        role="combobox"
        aria-label={t("model")}
        autoComplete="off"
        aria-autocomplete="list"
        aria-expanded={expanded}
        aria-controls={expanded ? listId : undefined}
        aria-activedescendant={activeId}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        onFocus={(event) => { showSuggestions(event.currentTarget); setActiveIndex(-1); }}
        onClick={(event) => showSuggestions(event.currentTarget)}
        onBlur={() => { setOpen(false); setActiveIndex(-1); }}
        onChange={(event) => {
          onChange(event.target.value);
          showSuggestions(event.currentTarget);
          setActiveIndex(-1);
        }}
        onKeyDown={onKeyDown}
      />
      {expanded && (
        <div className={cx(
          "absolute left-0 z-20 w-full min-w-0 rounded-md border border-field-line bg-panel shadow-lg",
          openAbove ? "bottom-full mb-1" : "top-full mt-1",
        )}>
          <div id={listId} role="listbox" aria-label={t("modelSuggestions")} className="max-h-52 overflow-y-auto p-1">
            {matches.map((model, index) => (
              <div
                key={model}
                id={`${listId}-${index}`}
                role="option"
                aria-selected={index === activeIndex}
                className={cx(
                  "cursor-pointer rounded px-2.5 py-2 font-mono text-sm break-all text-ink hover:bg-selected",
                  index === activeIndex && "bg-selected",
                )}
                onPointerDown={(event) => event.preventDefault()}
                onClick={(event) => { event.preventDefault(); select(model); }}
              >
                {model}
              </div>
            ))}
          </div>
          {matches.length === 0 && (
            <p role="status" className="px-3 py-2 text-sm text-muted">{t("noMatchingModels")}</p>
          )}
        </div>
      )}
    </div>
  );
}

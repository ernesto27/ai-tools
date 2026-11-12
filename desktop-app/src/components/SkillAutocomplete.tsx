import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "./ui";
import { SuggestionList } from "./SuggestionList";
import { useListboxNavigation } from "../hooks/useListboxNavigation";

interface Props {
  id: string;
  value: string;
  names: string[];
  disabled?: boolean;
  onChange: (value: string) => void;
}

export function SkillAutocomplete({ id, value, names, disabled = false, onChange }: Props) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const matches = names.filter((name) => name.toLowerCase().includes(query.trim().toLowerCase()));
  const choices = ["", ...matches];
  const unavailable = !disabled && !!value && !names.includes(value);
  const listbox = useListboxNavigation({
    id,
    optionCount: choices.length,
    disabled,
    onOpen: () => setQuery(""),
    onClose: () => setQuery(""),
    onSelect: (index) => {
      if (choices[index] !== value) onChange(choices[index]);
    },
  });

  return (
    <div className="relative min-w-0 w-full">
      <Input
        id={id}
        mono
        className="min-w-0 w-full"
        role="combobox"
        aria-label={t("customSkill")}
        aria-autocomplete="list"
        aria-expanded={listbox.expanded}
        aria-controls={listbox.expanded ? listbox.listId : undefined}
        aria-activedescendant={listbox.activeId}
        aria-describedby={unavailable ? `${id}-unavailable` : undefined}
        autoComplete="off"
        value={listbox.expanded ? query : value}
        placeholder={value || t("noneSkill")}
        disabled={disabled}
        onFocus={(event) => { listbox.show(event.currentTarget); listbox.resetHighlight(); }}
        onClick={(event) => listbox.show(event.currentTarget)}
        onBlur={listbox.close}
        onChange={(event) => {
          listbox.show(event.currentTarget);
          setQuery(event.target.value);
          listbox.resetHighlight();
        }}
        onKeyDown={listbox.onKeyDown}
      />
      {unavailable && <span id={`${id}-unavailable`} role="status" className="text-xs text-muted">{t("skillUnavailable")}</span>}
      {listbox.expanded && (
        <SuggestionList
          id={listbox.listId}
          label={t("skillSuggestions")}
          options={choices.map((name) => ({ value: name, label: name || t("noneSkill") }))}
          activeIndex={listbox.activeIndex}
          selectedIndex={choices.indexOf(value)}
          openAbove={listbox.openAbove}
          message={matches.length === 0 ? t(names.length ? "noMatchingSkills" : "noSkillsFound") : undefined}
          onSelect={listbox.select}
        />
      )}
    </div>
  );
}

import { useTranslation } from "react-i18next";
import { MODEL_CATALOG } from "../modelCatalog";
import { Input } from "./ui";
import { SuggestionList } from "./SuggestionList";
import { useListboxNavigation } from "../hooks/useListboxNavigation";

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
  const models = Object.prototype.hasOwnProperty.call(MODEL_CATALOG, agent)
    ? MODEL_CATALOG[agent]
    : [];
  const query = value.trim().toLowerCase();
  const matches = models.filter((model) => model.toLowerCase().includes(query));
  const listbox = useListboxNavigation({
    id,
    optionCount: matches.length,
    disabled: disabled || models.length === 0,
    onSelect: (index) => onChange(matches[index]),
  });

  return (
    <div className="relative min-w-0 w-full">
      <Input
        id={id}
        className="min-w-0 w-full"
        role="combobox"
        aria-label={t("model")}
        autoComplete="off"
        aria-autocomplete="list"
        aria-expanded={listbox.expanded}
        aria-controls={listbox.expanded ? listbox.listId : undefined}
        aria-activedescendant={listbox.activeId}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        onFocus={(event) => { listbox.show(event.currentTarget); listbox.resetHighlight(); }}
        onClick={(event) => listbox.show(event.currentTarget)}
        onBlur={listbox.close}
        onChange={(event) => {
          onChange(event.target.value);
          listbox.show(event.currentTarget);
          listbox.resetHighlight();
        }}
        onKeyDown={listbox.onKeyDown}
      />
      {listbox.expanded && (
        <SuggestionList
          id={listbox.listId}
          label={t("modelSuggestions")}
          options={matches.map((model) => ({ value: model, label: model }))}
          activeIndex={listbox.activeIndex}
          selectedIndex={listbox.activeIndex}
          openAbove={listbox.openAbove}
          message={matches.length === 0 ? t("noMatchingModels") : undefined}
          onSelect={listbox.select}
        />
      )}
    </div>
  );
}

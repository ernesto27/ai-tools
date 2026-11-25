import { AutocompleteInput } from "./AutocompleteInput";
import { SuggestionList } from "./SuggestionList";
import { useListboxNavigation } from "../hooks/useListboxNavigation";

interface Props {
  id: string;
  label: string;
  suggestionsLabel: string;
  emptyMessage: string;
  options: readonly string[];
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  suggestionsEnabled?: boolean;
  mono?: boolean;
}

export function TextAutocomplete({
  id, label, suggestionsLabel, emptyMessage, options, value, onChange, placeholder,
  disabled = false, suggestionsEnabled = true, mono = false,
}: Props) {
  const query = value.trim().toLowerCase();
  const matches = options.filter((option) => option.toLowerCase().includes(query));
  const listbox = useListboxNavigation({
    id,
    optionCount: matches.length,
    disabled: disabled || !suggestionsEnabled,
    onSelect: (index) => onChange(matches[index]),
  });

  return (
    <div className="relative min-w-0 w-full">
      <AutocompleteInput
        id={id}
        mono={mono}
        role="combobox"
        fieldLabel={label}
        autoComplete="off"
        aria-autocomplete="list"
        aria-expanded={listbox.expanded}
        aria-controls={listbox.expanded ? listbox.listId : undefined}
        aria-activedescendant={listbox.activeId}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        onClear={value ? () => {
          onChange("");
          listbox.close();
        } : undefined}
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
          label={suggestionsLabel}
          options={matches.map((option) => ({ value: option, label: option }))}
          activeIndex={listbox.activeIndex}
          selectedIndex={listbox.activeIndex}
          openAbove={listbox.openAbove}
          message={matches.length === 0 ? emptyMessage : undefined}
          onSelect={listbox.select}
        />
      )}
    </div>
  );
}

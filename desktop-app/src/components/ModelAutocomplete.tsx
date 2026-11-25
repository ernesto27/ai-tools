import { useTranslation } from "react-i18next";
import { MODEL_CATALOG } from "../modelCatalog";
import { TextAutocomplete } from "./TextAutocomplete";

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
  return (
    <TextAutocomplete
      id={id}
      label={t("model")}
      suggestionsLabel={t("modelSuggestions")}
      emptyMessage={t("noMatchingModels")}
      options={models}
      value={value}
      placeholder={placeholder}
      disabled={disabled}
      suggestionsEnabled={models.length > 0}
      onChange={onChange}
    />
  );
}

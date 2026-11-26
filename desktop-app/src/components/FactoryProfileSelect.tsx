import { useTranslation } from "react-i18next";
import type { FactoryProfile } from "../factoryProfiles";
import { Select } from "./ui";

interface Props {
  profiles: FactoryProfile[];
  value: string;
  disabled?: boolean;
  onChange: (id: string) => void;
}

export function FactoryProfileSelect({ profiles, value, disabled, onChange }: Props) {
  const { t } = useTranslation();
  return (
    <label className="flex min-w-0 items-center gap-3">
      <span>{t("factoryProfile")}</span>
      <Select id="factory-profile-select" className="min-w-0 max-w-full" value={value} disabled={disabled}
        onChange={(event) => onChange(event.target.value)}>
        <option value="">{t("repositoryDefaultProfile")}</option>
        {profiles.map(({ id, name }) => <option key={id} value={id}>{name}</option>)}
        {value && !profiles.some(({ id }) => id === value) && <option value={value}>{value}</option>}
      </Select>
    </label>
  );
}

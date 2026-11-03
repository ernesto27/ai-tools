import { useTranslation } from "react-i18next";
import { Field, Select, Title } from "../components/ui";
import { setLanguage, type Language } from "../i18n";

export function Settings() {
  const { t, i18n } = useTranslation();
  return (
    <div className="flex max-w-[680px] flex-col gap-9">
      <header className="flex flex-col gap-1">
        <Title>{t("Settings")}</Title>
        <p className="text-base text-muted">{t("Desktop preferences")}</p>
      </header>
      <section className="flex max-w-sm flex-col gap-3">
        <Field label={t("Language")} hint={t("Applies immediately and is saved on this device.")}>
          <Select id="language" value={i18n.resolvedLanguage === "es-AR" ? "es-AR" : "en"}
            onChange={(event) => {
              const value = event.target.value;
              const language: Language = value === "es-AR" ? "es-AR" : "en";
              void setLanguage(language);
            }}>
            <option value="en" lang="en">English</option>
            <option value="es-AR" lang="es-AR">Español (Argentina)</option>
          </Select>
        </Field>
      </section>
    </div>
  );
}

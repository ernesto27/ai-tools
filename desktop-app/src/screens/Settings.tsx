import { useTranslation } from "react-i18next";
import { Field, Select, Title, TextButton } from "../components/ui";
import { setLanguage, type Language } from "../i18n";

import { ThemeToggle } from "../components/ThemeToggle";
import type { Theme } from "../theme";

interface Props {
  theme: Theme;
  onToggleTheme: () => void;
  onBack: () => void;
}

export function Settings({ theme, onToggleTheme, onBack }: Props) {
  const { t, i18n } = useTranslation();
  return (
    <div className="flex max-w-[680px] flex-col gap-9">
      <TextButton data-testid="settings-back" className="self-start text-sm text-muted" onClick={onBack}>{t("back")}</TextButton>
      <header className="flex flex-col gap-1">
        <Title>{t("settings")}</Title>
        <p className="text-base text-muted">{t("desktopPreferences")}</p>
      </header>
      <section aria-label={t("appearance")} className="flex max-w-sm flex-col gap-3">
        <h2 className="text-md font-medium">{t("appearance")}</h2>
        <div className="rounded-md border border-line p-2">
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        </div>
      </section>
      <section className="flex max-w-sm flex-col gap-3">
        <Field label={t("language")} hint={t("appliesImmediatelyAndIsSavedOnThisDevice")}>
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

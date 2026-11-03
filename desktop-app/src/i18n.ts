import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./locales/en.json";
import esAR from "./locales/es-AR.json";

export type Language = "en" | "es-AR";
export const LANGUAGE_KEY = "soft-factory.language";

export function readLanguagePreference(): Language {
  try {
    const value = window.localStorage.getItem(LANGUAGE_KEY);
    return value === "es-AR" ? "es-AR" : "en";
  } catch {
    return "en";
  }
}

function applyDocumentLanguage(language: string) {
  document.documentElement.lang = language === "es-AR" ? "es-AR" : "en";
}

i18n.on("languageChanged", applyDocumentLanguage);

export const i18nReady = i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, "es-AR": { translation: esAR } },
  lng: readLanguagePreference(),
  fallbackLng: "en",
  supportedLngs: ["en", "es-AR"],
  load: "currentOnly",
  keySeparator: false,
  nsSeparator: false,
  interpolation: { escapeValue: false },
  returnEmptyString: false,
});

export async function setLanguage(language: Language): Promise<void> {
  await i18n.changeLanguage(language);
  try {
    window.localStorage.setItem(LANGUAGE_KEY, language);
  } catch (error) {
    console.warn("Could not save language preference", error);
  }
}

export default i18n;

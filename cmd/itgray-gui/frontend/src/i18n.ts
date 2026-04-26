import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import LngDetector from "i18next-browser-languagedetector";
import en from "./locales/en.json";
import ru from "./locales/ru.json";

// Initialize i18next once at app boot. The browser-language-detector picks
// the OS/browser locale on first run; the user can override it later via
// Settings → General, which persists the choice through SettingsService.
// Russian and English share an identical key tree — see locales/*.json.
i18n
  .use(LngDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      ru: { translation: ru },
    },
    fallbackLng: "en",
    supportedLngs: ["en", "ru"],
    interpolation: { escapeValue: false },
    detection: {
      order: ["localStorage", "navigator"],
      caches: ["localStorage"],
    },
  });

export default i18n;

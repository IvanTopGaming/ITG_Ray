// cmd/itgray-electron/frontend/src/i18n.ts
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./i18n/en.json";
import ru from "./i18n/ru.json";

// Single default namespace. useSettings().language is the source of truth;
// AppShell calls i18n.changeLanguage on change — no browser detector.
void i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, ru: { translation: ru } },
  lng: "en",
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  returnNull: false,
  // Resources are bundled synchronously (inline JSON, no backend), so i18n
  // is initialized before first render and Suspense never triggers. Disable
  // it explicitly so a future lazy-namespace migration can't silently
  // introduce a Suspense boundary the app isn't wrapped for.
  react: { useSuspense: false },
});

export default i18n;

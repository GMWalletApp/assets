import i18n from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { initReactI18next } from "react-i18next";
import enUS from "@/locales/en-US.json";
import zhCN from "@/locales/zh-CN.json";

const LANGUAGE_STORAGE_KEY = "gmwallet-assets-language";

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      "en-US": { translation: enUS },
      "zh-CN": { translation: zhCN },
    },
    supportedLngs: ["en-US", "zh-CN"],
    fallbackLng: "en-US",
    detection: {
      order: ["localStorage", "navigator"],
      caches: ["localStorage"],
      lookupLocalStorage: LANGUAGE_STORAGE_KEY,
      convertDetectedLanguage: (language) =>
        language.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US",
    },
    interpolation: { escapeValue: false },
  });

function applyDocumentLanguage(language: string) {
  document.documentElement.lang = language === "zh-CN" ? "zh-CN" : "en-US";
}

applyDocumentLanguage(i18n.resolvedLanguage ?? "en-US");
i18n.on("languageChanged", applyDocumentLanguage);

export default i18n;

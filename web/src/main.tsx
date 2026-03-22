import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";

if (import.meta.env.PROD) {
  console.log(
    '%c🦎 llm.log',
    'font-size: 24px; font-weight: bold; color: #10b981;'
  );
  console.log(
    '%cMonitoring your LLM costs so you don\'t have to check your bank account.',
    'font-size: 12px; color: #9ca3af;'
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

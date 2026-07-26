import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { ToastProvider } from "@/components";
import { I18nProvider } from "@/lib/i18n";
import { AuthGate } from "@/pages/AuthGate";
import { AccountsPage } from "@/pages/features/AccountsPage";
import { LocalKeysPage } from "@/pages/features/LocalKeysPage";
import { LogsPage } from "@/pages/features/LogsPage";
import { ModelsPage } from "@/pages/features/ModelsPage";
import { OverviewPage } from "@/pages/features/OverviewPage";
import { QuotasPage } from "@/pages/features/QuotasPage";
import { SettingsPage } from "@/pages/features/SettingsPage";
import { ProxiesPage } from "@/pages/features/ProxiesPage";
import { KeyPoolPage } from "@/pages/features/KeyPoolPage";
import { LoginPage } from "@/pages/LoginPage";
import { ShellLayout } from "@/pages/ShellLayout";

export function App() {
  return (
    <I18nProvider>
      <ToastProvider>
        <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/app"
            element={
              <AuthGate>
                <ShellLayout />
              </AuthGate>
            }
          >
            <Route index element={<Navigate to="overview" replace />} />
            <Route path="overview" element={<OverviewPage />} />
            <Route path="models" element={<ModelsPage />} />
            <Route path="local-keys" element={<LocalKeysPage />} />
            <Route path="key-pool" element={<KeyPoolPage />} />
            <Route path="zen-pool" element={<Navigate to="/app/key-pool" replace />} />
            <Route path="proxies" element={<ProxiesPage />} />
            <Route path="accounts" element={<AccountsPage />} />
            <Route path="quotas" element={<QuotasPage />} />
            <Route path="logs" element={<LogsPage />} />
            {/* legacy redirects */}
            <Route path="opencode-accounts" element={<Navigate to="/app/accounts" replace />} />
            <Route
              path="ollama-accounts"
              element={<Navigate to="/app/accounts?tab=ollama" replace />}
            />
            <Route
              path="ollama-quotas"
              element={<Navigate to="/app/quotas?tab=ollama" replace />}
            />
            <Route path="usage" element={<Navigate to="/app/logs?tab=usage-oc" replace />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
        </BrowserRouter>
      </ToastProvider>
    </I18nProvider>
  );
}

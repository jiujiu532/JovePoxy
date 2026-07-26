import { useEffect, useState } from "react";
import { Outlet, useNavigate } from "react-router-dom";
import { Sidebar, TopBar } from "@/components";
import { api } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { applyTheme, readTheme, type ThemeMode } from "@/lib/theme";

export function ShellLayout() {
  const navigate = useNavigate();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [theme, setTheme] = useState<ThemeMode>(() =>
    typeof window === "undefined" ? "light" : readTheme(),
  );

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  function handleLogout() {
    void api.logout().catch(() => undefined);
    setSessionHint(false);
    void navigate("/login");
  }

  return (
    <div className="relative flex h-[100dvh] max-h-[100dvh] overflow-hidden bg-paper-0">
      <Sidebar
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        theme={theme}
        onToggleTheme={() => {
          setTheme((t) => (t === "dark" ? "light" : "dark"));
        }}
        onLogout={handleLogout}
      />
      <div className="relative z-10 flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <TopBar onMenuClick={() => setSidebarOpen(true)} />
        <main className="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto p-3 sm:p-4 md:p-6">
          <div className="mx-auto w-full max-w-[1280px] min-w-0">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}

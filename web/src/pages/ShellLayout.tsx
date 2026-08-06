import { useEffect, useState } from "react";
import { Outlet, useNavigate } from "react-router-dom";
import { Sidebar, TopBar } from "@/components";
import { api } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { applyTheme, readTheme, type ThemeMode } from "@/lib/theme";

/**
 * App shell: fixed sidebar + fixed top title bar + scrollable content only.
 */
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

  function handleToggleTheme() {
    setTheme((t) => (t === "dark" ? "light" : "dark"));
  }

  return (
    <div className="relative flex h-[100dvh] max-h-[100dvh] overflow-hidden bg-paper-0">
      <Sidebar
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        onLogout={handleLogout}
      />
      <div className="relative z-10 flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <TopBar
          onMenuClick={() => setSidebarOpen(true)}
          theme={theme}
          onToggleTheme={handleToggleTheme}
        />
        <main className="scrollbar-none min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto p-3 sm:p-4 md:p-6">
          <div className="mx-auto flex w-full max-w-[1280px] min-w-0 flex-col gap-4">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}

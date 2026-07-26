/** Client-side cache only. Real auth is the httpOnly jovepoxy_admin cookie. */
const CACHE_KEY = "jovepoxy_admin_session_hint";

export function hasSessionHint(): boolean {
  return sessionStorage.getItem(CACHE_KEY) === "1";
}

export function setSessionHint(value: boolean): void {
  if (value) {
    sessionStorage.setItem(CACHE_KEY, "1");
    return;
  }
  sessionStorage.removeItem(CACHE_KEY);
}

/** @deprecated use setSessionHint */
export function setLoggedIn(value: boolean): void {
  setSessionHint(value);
}

/** @deprecated use hasSessionHint */
export function isLoggedIn(): boolean {
  return hasSessionHint();
}

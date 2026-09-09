import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, setTokenGetter } from "./api";
import type { User } from "./types";

interface AuthState {
  user: User | null;
  token: string | null;
  ready: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthCtx = createContext<AuthState | null>(null);
const TOKEN_KEY = "ticketsale_token";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() =>
    localStorage.getItem(TOKEN_KEY)
  );
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);

  // api.ts'nin token'a erisebilmesi icin.
  setTokenGetter(() => localStorage.getItem(TOKEN_KEY));

  useEffect(() => {
    if (!token) {
      setReady(true);
      return;
    }
    api<User>("/me")
      .then(setUser)
      .catch(() => {
        localStorage.removeItem(TOKEN_KEY);
        setToken(null);
      })
      .finally(() => setReady(true));
  }, [token]);

  const handleAuth = useCallback(
    async (path: string, email: string, password: string) => {
      const res = await api<{ token: string; user: User }>(path, {
        method: "POST",
        body: { email, password },
      });
      localStorage.setItem(TOKEN_KEY, res.token);
      setToken(res.token);
      setUser(res.user);
    },
    []
  );

  const value = useMemo<AuthState>(
    () => ({
      user,
      token,
      ready,
      login: (e, p) => handleAuth("/auth/login", e, p),
      register: (e, p) => handleAuth("/auth/register", e, p),
      logout: () => {
        localStorage.removeItem(TOKEN_KEY);
        setToken(null);
        setUser(null);
      },
    }),
    [user, token, ready, handleAuth]
  );

  return <AuthCtx.Provider value={value}>{children}</AuthCtx.Provider>;
}

export function useAuth(): AuthState {
  const v = useContext(AuthCtx);
  if (!v) throw new Error("useAuth AuthProvider icinde kullanilmali");
  return v;
}

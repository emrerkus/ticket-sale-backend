import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";

export function AuthPage() {
  const { login, register, user } = useAuth();
  const nav = useNavigate();
  const loc = useLocation() as { state?: { from?: string } };
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (user) {
    nav(loc.state?.from ?? "/", { replace: true });
    return null;
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      if (mode === "login") await login(email, password);
      else await register(email, password);
      nav(loc.state?.from ?? "/", { replace: true });
    } catch (e) {
      setErr(String((e as Error).message));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="narrow">
      <h1>{mode === "login" ? "Giris" : "Kayit"}</h1>
      <form onSubmit={submit} className="form">
        <label>
          E-posta
          <input
            type="email"
            value={email}
            required
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        <label>
          Sifre {mode === "register" && <span className="muted">(en az 8 karakter)</span>}
          <input
            type="password"
            value={password}
            required
            minLength={mode === "register" ? 8 : undefined}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        {err && <p className="error">{err}</p>}
        <button disabled={busy}>{busy ? "..." : mode === "login" ? "Giris yap" : "Kayit ol"}</button>
      </form>
      <p className="muted">
        {mode === "login" ? "Hesabin yok mu? " : "Zaten uye misin? "}
        <button
          className="linkbtn"
          onClick={() => {
            setMode(mode === "login" ? "register" : "login");
            setErr(null);
          }}
        >
          {mode === "login" ? "Kayit ol" : "Giris yap"}
        </button>
      </p>
    </div>
  );
}

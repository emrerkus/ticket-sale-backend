import { useEffect, useState } from "react";

/** expiresAt'a kalan sureyi mm:ss olarak gosterir. Bitince onExpire cagirir. */
export function Countdown({
  expiresAt,
  onExpire,
}: {
  expiresAt: string;
  onExpire?: () => void;
}) {
  const [left, setLeft] = useState(() => remaining(expiresAt));

  useEffect(() => {
    const t = setInterval(() => {
      const r = remaining(expiresAt);
      setLeft(r);
      if (r <= 0) {
        clearInterval(t);
        onExpire?.();
      }
    }, 1000);
    return () => clearInterval(t);
  }, [expiresAt, onExpire]);

  if (left <= 0) return <span className="pill danger">suresi doldu</span>;
  const m = Math.floor(left / 60);
  const s = left % 60;
  return (
    <span className="pill">
      {m}:{String(s).padStart(2, "0")}
    </span>
  );
}

function remaining(iso: string): number {
  return Math.max(0, Math.floor((new Date(iso).getTime() - Date.now()) / 1000));
}

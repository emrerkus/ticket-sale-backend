import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import type { CartSeat } from "./types";

interface CartState {
  seats: CartSeat[];
  add: (s: CartSeat) => void;
  remove: (seatId: string) => void;
  clear: () => void;
  totalCents: number;
}

const CartCtx = createContext<CartState | null>(null);
const KEY = "ticketsale_cart";

function load(): CartSeat[] {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? "[]");
  } catch {
    return [];
  }
}

export function CartProvider({ children }: { children: ReactNode }) {
  const [seats, setSeats] = useState<CartSeat[]>(load);

  const persist = useCallback((next: CartSeat[]) => {
    setSeats(next);
    localStorage.setItem(KEY, JSON.stringify(next));
  }, []);

  const value = useMemo<CartState>(
    () => ({
      seats,
      add: (s) => persist([...seats.filter((x) => x.seatId !== s.seatId), s]),
      remove: (seatId) => persist(seats.filter((x) => x.seatId !== seatId)),
      clear: () => persist([]),
      totalCents: seats.reduce((sum, s) => sum + s.priceCents, 0),
    }),
    [seats, persist]
  );

  return <CartCtx.Provider value={value}>{children}</CartCtx.Provider>;
}

export function useCart(): CartState {
  const v = useContext(CartCtx);
  if (!v) throw new Error("useCart CartProvider icinde kullanilmali");
  return v;
}

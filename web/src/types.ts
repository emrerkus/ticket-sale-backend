export interface User {
  id: string;
  email: string;
  created_at: string;
}

export interface EventListItem {
  id: string;
  title: string;
  description: string;
  venue_name: string;
  starts_at: string;
  sales_start_at: string;
  sales_end_at: string;
  status: string;
}

export interface TicketType {
  id: string;
  name: string;
  price_cents: number;
  currency: string;
}

export interface SeatMapEntry {
  seat_id: string;
  section: string;
  row_label: string;
  seat_number: string;
  ticket_type_id: string;
  status: "available" | "held" | "sold";
}

export interface EventDetail extends EventListItem {
  venue: { id: string; name: string; address: string };
  ticket_types: TicketType[];
  seat_map: SeatMapEntry[];
}

export interface SeatHold {
  event_id: string;
  seat_id: string;
  holder_id: string;
  status: string;
  expires_at: string;
}

export interface OrderItem {
  id: string;
  event_seat_id: string;
  section: string;
  row_label: string;
  seat_number: string;
  unit_price_cents: number;
}

export interface Payment {
  id: string;
  order_id: string;
  provider: string;
  provider_ref: string;
  amount_cents: number;
  currency: string;
  status: "pending" | "succeeded" | "failed";
  created_at: string;
}

export interface Order {
  id: string;
  user_id: string;
  status: "pending" | "paid" | "cancelled" | "expired";
  total_cents: number;
  currency: string;
  expires_at: string;
  created_at: string;
}

export interface OrderDetail extends Order {
  items: OrderItem[];
  payment?: Payment;
}

/** Sepetteki bir koltuk (tarayicida tutulur). */
export interface CartSeat {
  eventId: string;
  eventTitle: string;
  seatId: string;
  section: string;
  row: string;
  number: string;
  priceCents: number;
  currency: string;
  expiresAt: string;
}

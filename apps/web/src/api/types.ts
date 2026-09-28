export type RaffleStatus = "draft" | "open" | "closed";

export type Raffle = {
  id: string;
  slug: string;
  title: string;
  description: string;
  ticket_price_cents: number;
  total_tickets: number;
  status: string;
  available_tickets: number;
};

export type Ticket = {
  number: string;
};

export type Payment = {
  payment_id: string;
  qr_code_payload: string;
};

export type Order = {
  order_id: string;
  raffle_slug: string;
  email: string;
  quantity: number;
  amount_cents: number;
  status: string;
  expires_at: string;
  payment: Payment;
  tickets: Ticket[];
};

export type Purchase = {
  order_id: string;
  raffle_slug: string;
  raffle_title: string;
  quantity: number;
  amount_cents: number;
  status: string;
  tickets: Ticket[];
};

export type PurchasesResponse = {
  email: string;
  purchases: Purchase[];
};

export type PublicRaffleStatus = "open" | "closed";

export type CreateRaffleInput = {
  slug: string;
  title: string;
  description: string;
  ticket_price_cents: number;
  total_tickets: number;
};

import { newEventId } from "../lib/async";
import type {
  CreateRaffleInput,
  Order,
  Purchase,
  PurchasesResponse,
  PublicRaffleStatus,
  Raffle,
  RaffleStatus,
  Ticket,
} from "./types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export function apiBaseUrl(): string {
  const raw = import.meta.env.VITE_API_BASE_URL;
  const configured = typeof raw === "string" ? raw.trim() : "";
  // Unset keeps the laptop default. "/" (Dokploy) strips to "" so fetch stays
  // on this origin and nginx proxies /v1 to the api service.
  const base = configured.length > 0 ? configured : "http://localhost:8080";
  return base.replace(/\/+$/, "");
}

function fallbackMessage(status: number): string {
  switch (status) {
    case 400:
      return "Os dados enviados são inválidos.";
    case 401:
      return "Credencial recusada.";
    case 404:
      return "Não encontrado.";
    case 409:
      return "Não foi possível concluir: sem estoque, rifa fechada ou pedido expirado.";
    case 0:
      return "Não foi possível falar com a API.";
    default:
      return `A API retornou um erro (${status}).`;
  }
}

function parseJson(text: string): unknown {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

function readError(payload: unknown, status: number): { code: string; message: string } {
  if (typeof payload === "object" && payload !== null) {
    const body = payload as { code?: unknown; message?: unknown };
    const message =
      typeof body.message === "string" && body.message.trim().length > 0
        ? body.message
        : fallbackMessage(status);
    const code = typeof body.code === "string" && body.code.length > 0 ? body.code : "error";
    return { code, message };
  }
  return { code: "error", message: fallbackMessage(status) };
}

async function request(path: string, init: RequestInit = {}): Promise<unknown> {
  const headers = new Headers(init.headers);
  if (!headers.has("Accept")) headers.set("Accept", "application/json");
  if (init.body != null && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  let response: Response;
  try {
    response = await fetch(`${apiBaseUrl()}${path}`, {
      ...init,
      headers,
      cache: "no-store",
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    throw new ApiError(0, "network", "Não foi possível falar com a API.");
  }

  const text = await response.text();
  const payload = text.length > 0 ? parseJson(text) : null;
  if (!response.ok) {
    const errorBody = readError(payload, response.status);
    throw new ApiError(response.status, errorBody.code, errorBody.message);
  }
  return payload;
}

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof DOMException && error.name === "AbortError") return "";
  return "Não foi possível falar com a API.";
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== "object" || value === null) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return "";
}

function asNumber(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim().length > 0) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
}

function asTickets(value: unknown): Ticket[] {
  if (!Array.isArray(value)) return [];
  const tickets: Ticket[] = [];
  for (const item of value) {
    const record = asRecord(item);
    if (!record) continue;
    const number = record.number;
    if (typeof number !== "string" && typeof number !== "number") continue;
    tickets.push({ number: String(number) });
  }
  return tickets;
}

function invalid(message: string): ApiError {
  return new ApiError(0, "invalid_response", message);
}

export function parseRaffle(value: unknown): Raffle {
  const record = asRecord(value);
  if (!record || asString(record.id).length === 0) {
    throw invalid("A API devolveu uma rifa inválida.");
  }
  return {
    id: asString(record.id),
    slug: asString(record.slug),
    title: asString(record.title),
    description: asString(record.description),
    ticket_price_cents: asNumber(record.ticket_price_cents),
    total_tickets: asNumber(record.total_tickets),
    status: asString(record.status),
    available_tickets: asNumber(record.available_tickets),
  };
}

function parseOrder(value: unknown): Order {
  const record = asRecord(value);
  const payment = record ? asRecord(record.payment) : null;
  if (!record || !payment || asString(record.order_id).length === 0) {
    throw invalid("A API devolveu um pedido inválido.");
  }
  if (asString(payment.payment_id).length === 0) {
    throw invalid("A API não devolveu o identificador do pagamento.");
  }
  return {
    order_id: asString(record.order_id),
    raffle_slug: asString(record.raffle_slug),
    email: asString(record.email),
    quantity: asNumber(record.quantity),
    amount_cents: asNumber(record.amount_cents),
    status: asString(record.status),
    expires_at: asString(record.expires_at),
    payment: {
      payment_id: asString(payment.payment_id),
      qr_code_payload: asString(payment.qr_code_payload),
    },
    tickets: asTickets(record.tickets),
  };
}

function parsePurchase(value: unknown): Purchase | null {
  const record = asRecord(value);
  if (!record || asString(record.order_id).length === 0) return null;
  return {
    order_id: asString(record.order_id),
    raffle_slug: asString(record.raffle_slug),
    raffle_title: asString(record.raffle_title),
    quantity: asNumber(record.quantity),
    amount_cents: asNumber(record.amount_cents),
    status: asString(record.status),
    tickets: asTickets(record.tickets),
  };
}

function parsePurchases(value: unknown): PurchasesResponse {
  const record = asRecord(value);
  if (!record || !Array.isArray(record.purchases)) {
    throw invalid("A API devolveu uma consulta inválida.");
  }
  const purchases: Purchase[] = [];
  for (const item of record.purchases) {
    const purchase = parsePurchase(item);
    if (purchase) purchases.push(purchase);
  }
  return {
    email: asString(record.email),
    purchases,
  };
}

function parseRaffleList(value: unknown): Raffle[] {
  const record = asRecord(value);
  if (!record || !Array.isArray(record.raffles)) {
    throw invalid("A API devolveu uma lista de rifas inválida.");
  }
  return record.raffles.map((item) => parseRaffle(item));
}

export async function listRaffles(status: PublicRaffleStatus, signal?: AbortSignal): Promise<Raffle[]> {
  const query = new URLSearchParams({ status });
  const payload = await request(`/v1/raffles?${query.toString()}`, { signal });
  return parseRaffleList(payload);
}

export async function getRaffle(slug: string, signal?: AbortSignal): Promise<Raffle> {
  const payload = await request(`/v1/raffles/${encodeURIComponent(slug)}`, { signal });
  return parseRaffle(payload);
}

export async function createOrder(
  slug: string,
  email: string,
  quantity: number,
  signal?: AbortSignal,
): Promise<Order> {
  const payload = await request(`/v1/raffles/${encodeURIComponent(slug)}/orders`, {
    method: "POST",
    signal,
    body: JSON.stringify({ email, quantity }),
  });
  return parseOrder(payload);
}

export async function confirmPayment(
  secret: string,
  paymentId: string,
  signal?: AbortSignal,
): Promise<void> {
  await request("/v1/payments/webhooks", {
    method: "POST",
    signal,
    headers: { "X-Webhook-Secret": secret },
    body: JSON.stringify({
      event_id: newEventId(),
      payment_id: paymentId,
      status: "paid",
    }),
  });
}

export async function getPurchases(email: string, signal?: AbortSignal): Promise<PurchasesResponse> {
  const query = new URLSearchParams({ email });
  const payload = await request(`/v1/purchases?${query.toString()}`, { signal });
  return parsePurchases(payload);
}

function bearer(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}

export async function createRaffle(
  token: string,
  input: CreateRaffleInput,
  signal?: AbortSignal,
): Promise<Raffle> {
  const payload = await request("/v1/admin/raffles", {
    method: "POST",
    signal,
    headers: bearer(token),
    body: JSON.stringify(input),
  });
  return parseRaffle(payload);
}

export async function updateRaffleStatus(
  token: string,
  id: string,
  status: RaffleStatus,
  signal?: AbortSignal,
): Promise<Raffle> {
  const payload = await request(`/v1/admin/raffles/${encodeURIComponent(id)}`, {
    method: "PATCH",
    signal,
    headers: bearer(token),
    body: JSON.stringify({ status }),
  });
  return parseRaffle(payload);
}

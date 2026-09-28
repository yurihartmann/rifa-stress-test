const currency = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

const dateTime = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "short",
  timeStyle: "short",
});

export function formatCents(cents: number): string {
  if (!Number.isFinite(cents)) return "—";
  return currency.format(cents / 100);
}

export function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return dateTime.format(date);
}

export function formatTicketNumber(number: string, totalTickets: number | undefined): string {
  if (totalTickets === undefined || !Number.isFinite(totalTickets) || totalTickets < 1) {
    return number;
  }
  if (!/^\d+$/.test(number)) return number;
  const width = String(Math.trunc(totalTickets)).length;
  return number.padStart(width, "0");
}

export function raffleStatusLabel(status: string): string {
  switch (status) {
    case "draft":
      return "Rascunho";
    case "open":
      return "Aberta";
    case "closed":
      return "Fechada";
    default:
      return status;
  }
}

export function orderStatusLabel(status: string): string {
  switch (status) {
    case "pending_payment":
      return "Aguardando pagamento";
    case "paid":
      return "Pago";
    case "expired":
      return "Expirado";
    default:
      return status;
  }
}

export function availableTicketsLabel(count: number): string {
  if (count <= 0) return "Nenhum bilhete disponível";
  if (count === 1) return "1 bilhete disponível";
  return `${count} bilhetes disponíveis`;
}

export function normalizeEmail(value: string): string {
  return value.trim().toLowerCase();
}

export function readPositiveInt(value: string): number | null {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) return null;
  const parsed = Number(trimmed);
  if (!Number.isSafeInteger(parsed) || parsed < 1) return null;
  return parsed;
}

export function parseQuantity(value: string | null): number {
  if (value === null) return 1;
  return readPositiveInt(value) ?? 1;
}

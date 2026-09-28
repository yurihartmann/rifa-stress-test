import type { Ticket } from "../api/types";
import { formatTicketNumber } from "../lib/format";

type TicketListProps = {
  tickets: Ticket[];
  totalTickets?: number;
};

export function TicketList({ tickets, totalTickets }: TicketListProps) {
  if (tickets.length === 0) {
    return <p className="muted">Nenhum bilhete emitido ainda.</p>;
  }

  const sorted = [...tickets].sort((left, right) => {
    const a = formatTicketNumber(left.number, totalTickets);
    const b = formatTicketNumber(right.number, totalTickets);
    return a.localeCompare(b, "pt-BR", { numeric: true });
  });

  return (
    <ul className="ticket-list" aria-label="Bilhetes">
      {sorted.map((ticket, index) => (
        <li key={`${ticket.number}-${index}`}>
          {formatTicketNumber(ticket.number, totalTickets)}
        </li>
      ))}
    </ul>
  );
}

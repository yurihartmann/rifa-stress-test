import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { errorMessage, getRaffle } from "../api/client";
import type { Raffle } from "../api/types";
import { Field } from "../components/Field";
import { PageStatus } from "../components/PageStatus";
import { StatusBadge } from "../components/StatusBadge";
import {
  availableTicketsLabel,
  formatCents,
  raffleStatusLabel,
} from "../lib/format";
import type { LoadState } from "../lib/load";
import { usePageTitle } from "../lib/usePageTitle";

export default function RafflePage() {
  const { slug = "" } = useParams();
  const [state, setState] = useState<LoadState<Raffle>>({ status: "loading" });
  const [quantity, setQuantity] = useState(1);

  usePageTitle(state.status === "ready" ? state.data.title : "Rifa");

  useEffect(() => {
    const controller = new AbortController();
    setState({ status: "loading" });
    setQuantity(1);
    getRaffle(slug, controller.signal)
      .then((raffle) => {
        if (controller.signal.aborted) return;
        setState({ status: "ready", data: raffle });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setState({ status: "error", message: errorMessage(error) });
      });
    return () => controller.abort();
  }, [slug]);

  if (state.status === "loading") {
    return (
      <>
        <p className="eyebrow">Vitrine</p>
        <h1>Rifa</h1>
        <PageStatus kind="loading" title="Carregando a rifa" detail="Buscando os dados públicos." />
      </>
    );
  }

  if (state.status === "error" || state.status === "idle") {
    return (
      <>
        <p className="eyebrow">Vitrine</p>
        <h1>Rifa</h1>
        <PageStatus
          kind="error"
          title="Rifa indisponível"
          detail={state.status === "error" ? state.message : "Não foi possível carregar a rifa."}
        />
      </>
    );
  }

  const raffle = state.data;
  const checkoutTo = `/rifa/${raffle.slug.length > 0 ? raffle.slug : slug}/checkout?${new URLSearchParams({
    quantity: String(quantity),
  }).toString()}`;

  return (
    <>
      <p className="eyebrow">Vitrine</p>
      <article className="ticket">
        <div className="ticket-head">
          <h1>{raffle.title.length > 0 ? raffle.title : "Rifa sem título"}</h1>
          <StatusBadge status={raffle.status} label={raffleStatusLabel(raffle.status)} />
        </div>
        {raffle.description.trim().length > 0 ? (
          <p className="description">{raffle.description}</p>
        ) : (
          <p className="muted">Sem descrição.</p>
        )}
        <dl className="facts">
          <div>
            <dt>Preço</dt>
            <dd>{formatCents(raffle.ticket_price_cents)}</dd>
          </div>
          <div>
            <dt>Disponíveis</dt>
            <dd>{availableTicketsLabel(raffle.available_tickets)}</dd>
          </div>
          <div>
            <dt>Total</dt>
            <dd>{raffle.total_tickets}</dd>
          </div>
        </dl>
        {raffle.available_tickets <= 0 ? (
          <PageStatus
            kind="empty"
            title="Sem bilhetes"
            detail="Não há bilhetes disponíveis nesta rifa."
          />
        ) : null}
        {raffle.status !== "open" ? (
          <p className="note">
            Status atual: {raffleStatusLabel(raffle.status)}. Só uma rifa aberta aceita novos pedidos.
          </p>
        ) : null}
        <form
          className="stack"
          onSubmit={(event) => {
            event.preventDefault();
          }}
        >
          <Field label="Quantidade" hint="Mínimo de 1 bilhete.">
            <input
              type="number"
              min={1}
              step={1}
              inputMode="numeric"
              value={quantity}
              onChange={(event) => {
                const next = Number(event.target.value);
                if (!Number.isFinite(next)) return;
                setQuantity(Math.max(1, Math.trunc(next)));
              }}
            />
          </Field>
          {quantity > raffle.available_tickets ? (
            <p className="note">
              A quantidade passa dos bilhetes disponíveis. A API pode recusar o pedido.
            </p>
          ) : null}
          <Link className="button" to={checkoutTo}>
            Ir para o checkout
          </Link>
        </form>
      </article>
    </>
  );
}

import type { FormEvent } from "react";
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { errorMessage, listRaffles } from "../api/client";
import type { Raffle } from "../api/types";
import { Field } from "../components/Field";
import { PageStatus } from "../components/PageStatus";
import { StatusBadge } from "../components/StatusBadge";
import { availableTicketsLabel, formatCents, raffleStatusLabel } from "../lib/format";
import type { LoadState } from "../lib/load";
import { usePageTitle } from "../lib/usePageTitle";

type Catalog = {
  open: Raffle[];
  closed: Raffle[];
};

type SectionCopy = {
  id: string;
  title: string;
  emptyTitle: string;
  emptyDetail: string;
  raffles: Raffle[];
};

function raffleTitle(raffle: Raffle): string {
  return raffle.title.trim().length > 0 ? raffle.title : "Rifa sem título";
}

function RaffleSection({ id, title, emptyTitle, emptyDetail, raffles }: SectionCopy) {
  return (
    <section className="catalog" aria-labelledby={id}>
      <h2 id={id}>{title}</h2>
      {raffles.length === 0 ? (
        <PageStatus kind="empty" title={emptyTitle} detail={emptyDetail} />
      ) : (
        <ul className="raffle-list">
          {raffles.map((raffle) => {
            const titleText = raffleTitle(raffle);
            const detail =
              raffle.slug.length > 0 && !raffle.slug.includes("/") ? `/rifa/${raffle.slug}` : null;
            return (
              <li key={raffle.id}>
                <article className="ticket">
                  <div className="ticket-head">
                    <h3>
                      {detail !== null ? (
                        <Link className="raffle-title" to={detail}>
                          {titleText}
                        </Link>
                      ) : (
                        titleText
                      )}
                    </h3>
                    <StatusBadge status={raffle.status} label={raffleStatusLabel(raffle.status)} />
                  </div>
                  {raffle.description.trim().length > 0 ? (
                    <p className="description">{raffle.description}</p>
                  ) : null}
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
                  {detail !== null ? <Link to={detail}>Ver rifa</Link> : null}
                </article>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

export default function HomePage() {
  const navigate = useNavigate();
  const [slug, setSlug] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [catalog, setCatalog] = useState<LoadState<Catalog>>({ status: "loading" });
  usePageTitle("Início");

  useEffect(() => {
    const controller = new AbortController();
    setCatalog({ status: "loading" });
    Promise.all([
      listRaffles("open", controller.signal),
      listRaffles("closed", controller.signal),
    ])
      .then(([open, closed]) => {
        if (controller.signal.aborted) return;
        setCatalog({ status: "ready", data: { open, closed } });
      })
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        setCatalog({ status: "error", message: errorMessage(reason) });
      });
    return () => controller.abort();
  }, []);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = slug.trim();
    if (next.length === 0) {
      setError("Informe o slug da rifa.");
      return;
    }
    if (next.includes("/")) {
      setError("O slug não pode conter barra.");
      return;
    }
    setError(null);
    navigate(`/rifa/${next}`);
  }

  return (
    <>
      <p className="eyebrow">Vitrine</p>
      <h1>Rifas</h1>
      <p className="lede">Rifas abertas para compra e rifas já concluídas.</p>
      {catalog.status === "loading" ? (
        <PageStatus
          kind="loading"
          title="Carregando as rifas"
          detail="Buscando rifas abertas e concluídas."
        />
      ) : null}
      {catalog.status === "error" ? (
        <PageStatus
          kind="error"
          title="Não foi possível listar as rifas"
          detail={catalog.message.length > 0 ? catalog.message : "Não foi possível falar com a API."}
        />
      ) : null}
      {catalog.status === "ready" ? (
        <>
          <RaffleSection
            id="rifas-abertas"
            title="Abertas"
            emptyTitle="Nenhuma rifa aberta"
            emptyDetail="Quando uma rifa for aberta, ela aparece aqui."
            raffles={catalog.data.open}
          />
          <RaffleSection
            id="rifas-concluidas"
            title="Concluídas"
            emptyTitle="Nenhuma rifa concluída"
            emptyDetail="Rifas fechadas aparecem aqui depois de concluídas."
            raffles={catalog.data.closed}
          />
        </>
      ) : null}
      <section className="panel" aria-labelledby="abrir-slug">
        <h2 id="abrir-slug">Abrir pelo slug</h2>
        <p className="muted">Use o identificador público quando a rifa não estiver na lista.</p>
        {error !== null ? <PageStatus kind="error" title="Não foi possível abrir" detail={error} /> : null}
        <form className="stack" onSubmit={onSubmit}>
          <Field label="Slug" hint="O mesmo identificador publicado na rifa.">
            <input
              value={slug}
              onChange={(event) => setSlug(event.target.value)}
              placeholder="carro-ano-novo"
              autoComplete="off"
              spellCheck={false}
            />
          </Field>
          <button className="button" type="submit">
            Ver rifa
          </button>
        </form>
      </section>
    </>
  );
}

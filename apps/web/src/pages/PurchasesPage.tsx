import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { errorMessage, getPurchases, getRaffle } from "../api/client";
import type { Purchase, PurchasesResponse } from "../api/types";
import { Field } from "../components/Field";
import { PageStatus } from "../components/PageStatus";
import { StatusBadge } from "../components/StatusBadge";
import { TicketList } from "../components/TicketList";
import { formatCents, normalizeEmail, orderStatusLabel } from "../lib/format";
import type { LoadState } from "../lib/load";
import { usePageTitle } from "../lib/usePageTitle";

type RaffleGroup = {
  slug: string;
  title: string;
  purchases: Purchase[];
};

function groupByRaffle(purchases: Purchase[]): RaffleGroup[] {
  const groups = new Map<string, RaffleGroup>();
  for (const purchase of purchases) {
    const existing = groups.get(purchase.raffle_slug);
    if (existing) {
      existing.purchases.push(purchase);
      continue;
    }
    groups.set(purchase.raffle_slug, {
      slug: purchase.raffle_slug,
      title: purchase.raffle_title,
      purchases: [purchase],
    });
  }
  return [...groups.values()];
}

async function loadTicketWidths(
  slugs: string[],
  signal: AbortSignal,
): Promise<Record<string, number>> {
  const entries = await Promise.all(
    slugs.map(async (slug) => {
      try {
        const raffle = await getRaffle(slug, signal);
        return [slug, raffle.total_tickets] as const;
      } catch (error: unknown) {
        if (signal.aborted) throw error;
        return null;
      }
    }),
  );
  const widths: Record<string, number> = {};
  for (const entry of entries) {
    if (entry && entry[1] > 0) widths[entry[0]] = entry[1];
  }
  return widths;
}

export default function PurchasesPage() {
  const [params, setParams] = useSearchParams();
  const emailQuery = params.get("email") ?? "";
  const [email, setEmail] = useState(() => normalizeEmail(emailQuery));
  const [refreshKey, setRefreshKey] = useState(0);
  const [state, setState] = useState<LoadState<PurchasesResponse>>({ status: "idle" });
  const [widths, setWidths] = useState<Record<string, number>>({});
  usePageTitle("Compras");

  useEffect(() => {
    setEmail(normalizeEmail(emailQuery));
  }, [emailQuery]);

  useEffect(() => {
    const next = normalizeEmail(emailQuery);
    if (next.length === 0) {
      setState({ status: "idle" });
      setWidths({});
      return;
    }

    const controller = new AbortController();
    setState({ status: "loading" });
    getPurchases(next, controller.signal)
      .then(async (data) => {
        if (controller.signal.aborted) return;
        const slugs = [...new Set(data.purchases.map((purchase) => purchase.raffle_slug))];
        const nextWidths = slugs.length > 0 ? await loadTicketWidths(slugs, controller.signal) : {};
        if (controller.signal.aborted) return;
        setWidths(nextWidths);
        setState({ status: "ready", data });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setState({ status: "error", message: errorMessage(error) });
      });

    return () => controller.abort();
  }, [emailQuery, refreshKey]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = normalizeEmail(email);
    if (next.length === 0 || !next.includes("@")) {
      setState({ status: "error", message: "Informe um e-mail válido." });
      return;
    }
    setParams({ email: next });
    setRefreshKey((value) => value + 1);
  }

  const groups = state.status === "ready" ? groupByRaffle(state.data.purchases) : [];

  return (
    <>
      <p className="eyebrow">Consulta</p>
      <h1>Compras</h1>
      <p className="lede">Veja pedidos e bilhetes com o e-mail usado no checkout. Não há conta nem senha.</p>
      <form className="stack" onSubmit={onSubmit}>
        <Field label="E-mail">
          <input
            type="email"
            required
            autoComplete="email"
            autoCapitalize="off"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </Field>
        <button className="button" type="submit" disabled={state.status === "loading"}>
          {state.status === "loading" ? "Consultando…" : "Consultar"}
        </button>
      </form>

      {state.status === "idle" ? (
        <PageStatus
          kind="empty"
          title="Nenhuma consulta"
          detail="Informe um e-mail para listar pedidos e bilhetes."
        />
      ) : null}
      {state.status === "loading" ? (
        <PageStatus kind="loading" title="Buscando compras" detail="Consultando pedidos deste e-mail." />
      ) : null}
      {state.status === "error" ? (
        <PageStatus kind="error" title="Consulta não concluída" detail={state.message} />
      ) : null}
      {state.status === "ready" && state.data.purchases.length === 0 ? (
        <PageStatus
          kind="empty"
          title="Nenhuma compra"
          detail={`Não há pedidos para ${state.data.email.length > 0 ? state.data.email : emailQuery}.`}
        />
      ) : null}
      {state.status === "ready" && groups.length > 0 ? (
        <div className="stack">
          {groups.map((group) => (
            <section className="panel" key={group.slug.length > 0 ? group.slug : group.title}>
              <h2>{group.title.length > 0 ? group.title : "Rifa"}</h2>
              {group.slug.length > 0 ? (
                <p>
                  <Link to={`/rifa/${group.slug}`}>{group.slug}</Link>
                </p>
              ) : null}
              {group.purchases.map((purchase) => (
                <article className="purchase" key={purchase.order_id}>
                  <div className="ticket-head">
                    <h3>
                      {purchase.quantity === 1 ? "1 bilhete" : `${purchase.quantity} bilhetes`} ·{" "}
                      {formatCents(purchase.amount_cents)}
                    </h3>
                    <StatusBadge status={purchase.status} label={orderStatusLabel(purchase.status)} />
                  </div>
                  <p className="muted">
                    Pedido <code className="id">{purchase.order_id}</code>
                  </p>
                  <TicketList tickets={purchase.tickets} totalTickets={widths[group.slug]} />
                </article>
              ))}
            </section>
          ))}
        </div>
      ) : null}
    </>
  );
}

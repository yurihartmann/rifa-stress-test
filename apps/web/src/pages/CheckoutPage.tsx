import { lazy, Suspense, useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import {
  ApiError,
  confirmPayment,
  createOrder,
  errorMessage,
  getPurchases,
  getRaffle,
} from "../api/client";
import type { Order, Raffle, Ticket } from "../api/types";
import { Field } from "../components/Field";
import { PageStatus } from "../components/PageStatus";
import { StatusBadge } from "../components/StatusBadge";
import { TicketList } from "../components/TicketList";
import { delay } from "../lib/async";
import {
  formatCents,
  formatDateTime,
  normalizeEmail,
  orderStatusLabel,
  parseQuantity,
  raffleStatusLabel,
} from "../lib/format";
import type { LoadState } from "../lib/load";
import { sessionKeys, useSessionValue } from "../lib/session";
import { usePageTitle } from "../lib/usePageTitle";

const QrPayload = lazy(() => import("../components/QrPayload"));

const POLL_TIMEOUT_MS = 45_000;
const POLL_INTERVAL_MS = 1_000;

export default function CheckoutPage() {
  const { slug = "" } = useParams();
  const [params] = useSearchParams();
  const quantity = parseQuantity(params.get("quantity"));
  const [secret, setSecret] = useSessionValue(sessionKeys.webhookSecret);
  const [raffleState, setRaffleState] = useState<LoadState<Raffle>>({ status: "loading" });
  const [email, setEmail] = useState("");
  const [order, setOrder] = useState<Order | null>(null);
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [busy, setBusy] = useState<"create" | "pay" | "poll" | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const actionAbort = useRef<AbortController | null>(null);

  usePageTitle(raffleState.status === "ready" ? raffleState.data.title : "Checkout");

  useEffect(() => {
    const controller = new AbortController();
    setRaffleState({ status: "loading" });
    getRaffle(slug, controller.signal)
      .then((raffle) => {
        if (controller.signal.aborted) return;
        setRaffleState({ status: "ready", data: raffle });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setRaffleState({ status: "error", message: errorMessage(error) });
      });
    return () => controller.abort();
  }, [slug]);

  useEffect(() => {
    setOrder(null);
    setTickets([]);
    setBusy(null);
    setActionError(null);
    return () => {
      actionAbort.current?.abort();
      actionAbort.current = null;
    };
  }, [slug, quantity]);

  function beginAction(): AbortController {
    actionAbort.current?.abort();
    const controller = new AbortController();
    actionAbort.current = controller;
    return controller;
  }

  async function onCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = normalizeEmail(email);
    if (normalized.length === 0 || !normalized.includes("@")) {
      setActionError("Informe um e-mail válido.");
      return;
    }

    const controller = beginAction();
    setBusy("create");
    setActionError(null);
    setTickets([]);
    try {
      const created = await createOrder(slug, normalized, quantity, controller.signal);
      if (controller.signal.aborted) return;
      const withEmail = created.email.length > 0 ? created : { ...created, email: normalized };
      setOrder(withEmail);
      setTickets(withEmail.tickets);
      setBusy(null);
    } catch (error: unknown) {
      if (controller.signal.aborted) return;
      setBusy(null);
      setActionError(errorMessage(error));
    }
  }

  async function onSimulate() {
    if (order === null) return;
    const webhookSecret = secret.trim();
    if (webhookSecret.length === 0) {
      setActionError("Informe o segredo do webhook.");
      return;
    }

    const controller = beginAction();
    const paymentId = order.payment.payment_id;
    const orderId = order.order_id;
    const pollEmail = order.email;
    const expected = order.quantity;
    setBusy("pay");
    setActionError(null);

    try {
      await confirmPayment(webhookSecret, paymentId, controller.signal);
      if (controller.signal.aborted) return;
      setBusy("poll");
      const deadline = Date.now() + POLL_TIMEOUT_MS;

      while (!controller.signal.aborted) {
        const data = await getPurchases(pollEmail, controller.signal);
        if (controller.signal.aborted) return;
        const purchase = data.purchases.find((item) => item.order_id === orderId);
        if (purchase) {
          setTickets(purchase.tickets);
          setOrder((current) =>
            current !== null && current.order_id === orderId
              ? { ...current, status: purchase.status, tickets: purchase.tickets }
              : current,
          );
          if (purchase.status === "expired") {
            throw new ApiError(409, "order_expired", "O pedido expirou.");
          }
          if (expected > 0 && purchase.tickets.length >= expected) {
            setBusy(null);
            return;
          }
        }
        if (Date.now() >= deadline) {
          throw new ApiError(
            0,
            "tickets_pending",
            "O pagamento foi confirmado, mas os bilhetes ainda não foram emitidos. Consulte suas compras em instantes.",
          );
        }
        await delay(POLL_INTERVAL_MS, controller.signal);
      }
    } catch (error: unknown) {
      if (controller.signal.aborted || (error instanceof DOMException && error.name === "AbortError")) {
        return;
      }
      setBusy(null);
      setActionError(errorMessage(error));
    }
  }

  if (raffleState.status === "loading") {
    return (
      <>
        <p className="eyebrow">Checkout</p>
        <h1>Checkout</h1>
        <PageStatus kind="loading" title="Carregando a rifa" detail="Buscando preço e disponibilidade." />
      </>
    );
  }

  if (raffleState.status === "error" || raffleState.status === "idle") {
    return (
      <>
        <p className="eyebrow">Checkout</p>
        <h1>Checkout</h1>
        <PageStatus
          kind="error"
          title="Checkout indisponível"
          detail={
            raffleState.status === "error"
              ? raffleState.message
              : "Não foi possível carregar a rifa."
          }
        />
      </>
    );
  }

  const raffle = raffleState.data;
  const totalTickets = raffle.total_tickets > 0 ? raffle.total_tickets : undefined;
  const issued = order !== null && order.quantity > 0 && tickets.length >= order.quantity;
  const purchasesTo =
    order !== null ? `/compras?${new URLSearchParams({ email: order.email }).toString()}` : "/compras";

  return (
    <>
      <p className="eyebrow">Checkout</p>
      <h1>{raffle.title.length > 0 ? raffle.title : "Checkout"}</h1>
      <p className="lede">
        {quantity === 1 ? "1 bilhete" : `${quantity} bilhetes`} ·{" "}
        {formatCents(raffle.ticket_price_cents * quantity)}
      </p>
      <p>
        <Link to={`/rifa/${slug}`}>Alterar quantidade</Link>
      </p>
      {raffle.status !== "open" ? (
        <p className="note">
          Status atual: {raffleStatusLabel(raffle.status)}. Só uma rifa aberta aceita novos pedidos.
        </p>
      ) : null}

      {order === null ? (
        <section className="panel">
          <h2>Pagamento</h2>
          <PageStatus
            kind="empty"
            title="Nenhum pagamento gerado"
            detail="Informe só o e-mail para criar o pedido e receber o QR code."
          />
          <form className="stack" onSubmit={onCreate}>
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
            {actionError !== null ? (
              <PageStatus kind="error" title="Pedido não criado" detail={actionError} />
            ) : null}
            {busy === "create" ? (
              <PageStatus kind="loading" title="Criando pedido" detail="Reservando a quantidade escolhida." />
            ) : null}
            <button className="button" type="submit" disabled={busy !== null}>
              {busy === "create" ? "Criando pedido…" : "Criar pedido"}
            </button>
          </form>
        </section>
      ) : (
        <section className="panel">
          <div className="ticket-head">
            <h2>Pagamento</h2>
            <StatusBadge status={order.status} label={orderStatusLabel(order.status)} />
          </div>
          <dl className="facts">
            <div>
              <dt>Valor</dt>
              <dd>{formatCents(order.amount_cents)}</dd>
            </div>
            <div>
              <dt>E-mail</dt>
              <dd>{order.email}</dd>
            </div>
            <div>
              <dt>Expira</dt>
              <dd>{formatDateTime(order.expires_at)}</dd>
            </div>
          </dl>
          <p>
            Pagamento <code className="id">{order.payment.payment_id}</code>
          </p>
          {order.payment.qr_code_payload.length > 0 ? (
            <Suspense
              fallback={
                <PageStatus
                  kind="loading"
                  title="Desenhando o QR"
                  detail="Carregando o gerador de QR code."
                />
              }
            >
              <QrPayload value={order.payment.qr_code_payload} />
            </Suspense>
          ) : (
            <PageStatus
              kind="empty"
              title="Sem QR"
              detail="A API não devolveu o payload do QR code."
            />
          )}
          {order.payment.qr_code_payload.length > 0 ? (
            <details className="payload">
              <summary>Payload do QR</summary>
              <code>{order.payment.qr_code_payload}</code>
            </details>
          ) : null}

          {issued ? (
            <>
              <p className="success">Bilhetes emitidos.</p>
              <TicketList tickets={tickets} totalTickets={totalTickets} />
            </>
          ) : (
            <TicketList tickets={tickets} totalTickets={totalTickets} />
          )}

          {busy === "poll" ? (
            <PageStatus
              kind="loading"
              title="Emitindo bilhetes"
              detail={`${tickets.length} de ${order.quantity} ${order.quantity === 1 ? "bilhete" : "bilhetes"}.`}
            />
          ) : null}

          {actionError !== null ? (
            <PageStatus kind="error" title="Pagamento não concluído" detail={actionError} />
          ) : null}

          {issued ? null : (
            <form
              className="stack"
              onSubmit={(event) => {
                event.preventDefault();
                void onSimulate();
              }}
            >
              <Field
                label="Segredo do webhook (X-Webhook-Secret)"
                hint="Guardado só nesta sessão, no sessionStorage. Não fica no código."
              >
                <input
                  type="password"
                  autoComplete="off"
                  spellCheck={false}
                  value={secret}
                  onChange={(event) => setSecret(event.target.value)}
                />
              </Field>
              <button className="button" type="submit" disabled={busy !== null}>
                {busy === "pay"
                  ? "Enviando pagamento…"
                  : busy === "poll"
                    ? "Aguardando bilhetes…"
                    : "Simular pagamento"}
              </button>
            </form>
          )}

          <p>
            <Link to={purchasesTo}>Ver compras deste e-mail</Link>
          </p>
          <button
            className="button button-secondary"
            type="button"
            onClick={() => {
              actionAbort.current?.abort();
              setOrder(null);
              setTickets([]);
              setBusy(null);
              setActionError(null);
            }}
          >
            Fazer outro pedido
          </button>
        </section>
      )}
    </>
  );
}

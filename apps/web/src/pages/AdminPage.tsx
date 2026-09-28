import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  createRaffle,
  errorMessage,
  getRaffle,
  updateRaffleStatus,
} from "../api/client";
import type { Raffle, RaffleStatus } from "../api/types";
import { Field } from "../components/Field";
import { PageStatus } from "../components/PageStatus";
import { StatusBadge } from "../components/StatusBadge";
import {
  availableTicketsLabel,
  formatCents,
  raffleStatusLabel,
  readPositiveInt,
} from "../lib/format";
import { sessionKeys, useSessionValue } from "../lib/session";
import { usePageTitle } from "../lib/usePageTitle";

const STATUSES: { value: RaffleStatus; label: string }[] = [
  { value: "draft", label: "Rascunho" },
  { value: "open", label: "Aberta" },
  { value: "closed", label: "Fechada" },
];

export default function AdminPage() {
  const [token, setToken] = useSessionValue(sessionKeys.adminToken);
  const [slug, setSlug] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priceCents, setPriceCents] = useState("1000");
  const [totalTickets, setTotalTickets] = useState("100");
  const [loadSlug, setLoadSlug] = useState("");
  const [raffle, setRaffle] = useState<Raffle | null>(null);
  const [busy, setBusy] = useState<"create" | "load" | "status" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  usePageTitle("Admin");

  useEffect(() => {
    return () => abortRef.current?.abort();
  }, []);

  const parsedPrice = readPositiveInt(priceCents);

  function begin(): AbortController | null {
    const trimmed = token.trim();
    if (trimmed.length === 0) {
      setError("Informe o token de administrador.");
      setNotice(null);
      return null;
    }
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setError(null);
    setNotice(null);
    return controller;
  }

  async function onCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextSlug = slug.trim();
    const nextTitle = title.trim();
    const tickets = readPositiveInt(totalTickets);
    if (nextSlug.length === 0) {
      setError("Informe o slug.");
      return;
    }
    if (nextSlug.includes("/")) {
      setError("O slug não pode conter barra.");
      return;
    }
    if (nextTitle.length === 0) {
      setError("Informe o título.");
      return;
    }
    if (parsedPrice === null) {
      setError("Informe o preço em centavos, maior que zero.");
      return;
    }
    if (tickets === null) {
      setError("Informe o total de bilhetes, maior que zero.");
      return;
    }
    const controller = begin();
    if (controller === null) return;
    setBusy("create");
    try {
      const created = await createRaffle(
        token.trim(),
        {
          slug: nextSlug,
          title: nextTitle,
          description: description.trim(),
          ticket_price_cents: parsedPrice,
          total_tickets: tickets,
        },
        controller.signal,
      );
      if (controller.signal.aborted) return;
      setRaffle(created);
      setNotice("Rifa criada.");
      setBusy(null);
    } catch (caught: unknown) {
      if (controller.signal.aborted) return;
      setBusy(null);
      setError(errorMessage(caught));
    }
  }

  async function onLoad(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextSlug = loadSlug.trim();
    if (nextSlug.length === 0) {
      setError("Informe o slug para carregar a rifa.");
      setNotice(null);
      return;
    }
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setBusy("load");
    setError(null);
    setNotice(null);
    try {
      const loaded = await getRaffle(nextSlug, controller.signal);
      if (controller.signal.aborted) return;
      setRaffle(loaded);
      setNotice("Rifa carregada. O token só é enviado ao alterar o status.");
      setBusy(null);
    } catch (caught: unknown) {
      if (controller.signal.aborted) return;
      setBusy(null);
      setError(errorMessage(caught));
    }
  }

  async function onStatus(status: RaffleStatus) {
    if (raffle === null || raffle.status === status) return;
    const controller = begin();
    if (controller === null) return;
    setBusy("status");
    try {
      const updated = await updateRaffleStatus(token.trim(), raffle.id, status, controller.signal);
      if (controller.signal.aborted) return;
      setRaffle(updated);
      setNotice("Status atualizado.");
      setBusy(null);
    } catch (caught: unknown) {
      if (controller.signal.aborted) return;
      setBusy(null);
      setError(errorMessage(caught));
    }
  }

  const loadingDetail =
    busy === "create"
      ? "Criando a rifa."
      : busy === "load"
        ? "Buscando a rifa pelo slug."
        : "Atualizando o status.";

  return (
    <>
      <p className="eyebrow">Backoffice</p>
      <h1>Admin</h1>
      <p className="lede">
        O token fica só no sessionStorage deste separador e vai no header Authorization: Bearer.
      </p>
      <form
        className="stack"
        onSubmit={(event) => {
          event.preventDefault();
        }}
      >
        <Field label="Token de administrador" hint="Não é gravado no código nem no localStorage.">
          <input
            type="password"
            autoComplete="off"
            spellCheck={false}
            value={token}
            onChange={(event) => setToken(event.target.value)}
          />
        </Field>
      </form>

      <section className="panel">
        <h2>Criar rifa</h2>
        <form className="stack" onSubmit={onCreate}>
          <Field label="Slug">
            <input
              value={slug}
              onChange={(event) => setSlug(event.target.value)}
              autoComplete="off"
              spellCheck={false}
              placeholder="carro-ano-novo"
              required
            />
          </Field>
          <Field label="Título">
            <input value={title} onChange={(event) => setTitle(event.target.value)} required />
          </Field>
          <Field label="Descrição">
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              rows={4}
            />
          </Field>
          <Field
            label="Preço (centavos)"
            hint={parsedPrice !== null ? formatCents(parsedPrice) : "Inteiro maior que zero."}
          >
            <input
              type="number"
              min={1}
              step={1}
              inputMode="numeric"
              value={priceCents}
              onChange={(event) => setPriceCents(event.target.value)}
              required
            />
          </Field>
          <Field label="Total de bilhetes">
            <input
              type="number"
              min={1}
              step={1}
              inputMode="numeric"
              value={totalTickets}
              onChange={(event) => setTotalTickets(event.target.value)}
              required
            />
          </Field>
          <button className="button" type="submit" disabled={busy !== null}>
            {busy === "create" ? "Criando rifa…" : "Criar rifa"}
          </button>
        </form>
      </section>

      <section className="panel">
        <h2>Carregar rifa existente</h2>
        <form className="stack" onSubmit={onLoad}>
          <Field label="Slug" hint="A leitura é pública. A mudança de status exige o token.">
            <input
              value={loadSlug}
              onChange={(event) => setLoadSlug(event.target.value)}
              autoComplete="off"
              spellCheck={false}
            />
          </Field>
          <button className="button button-secondary" type="submit" disabled={busy !== null}>
            {busy === "load" ? "Carregando…" : "Carregar rifa"}
          </button>
        </form>
      </section>

      {busy !== null ? (
        <PageStatus kind="loading" title="Aguarde" detail={loadingDetail} />
      ) : null}
      {error !== null ? <PageStatus kind="error" title="Não foi possível concluir" detail={error} /> : null}
      {notice !== null ? <p className="success">{notice}</p> : null}

      {raffle !== null ? (
        <section className="panel">
          <div className="ticket-head">
            <h2>{raffle.title.length > 0 ? raffle.title : "Rifa"}</h2>
            <StatusBadge status={raffle.status} label={raffleStatusLabel(raffle.status)} />
          </div>
          <dl className="facts">
            <div>
              <dt>Slug</dt>
              <dd>{raffle.slug}</dd>
            </div>
            <div>
              <dt>Preço</dt>
              <dd>{formatCents(raffle.ticket_price_cents)}</dd>
            </div>
            <div>
              <dt>Bilhetes</dt>
              <dd>
                {availableTicketsLabel(raffle.available_tickets)} de {raffle.total_tickets}
              </dd>
            </div>
          </dl>
          <p>
            ID <code className="id">{raffle.id}</code>
          </p>
          {raffle.slug.length > 0 ? (
            <p>
              <Link to={`/rifa/${raffle.slug}`}>Abrir vitrine</Link>
            </p>
          ) : null}
          <div className="status-control" role="group" aria-label="Alterar status">
            {STATUSES.map((option) => (
              <button
                key={option.value}
                className={
                  raffle.status === option.value ? "button" : "button button-secondary"
                }
                type="button"
                aria-pressed={raffle.status === option.value}
                disabled={busy !== null || raffle.status === option.value}
                onClick={() => {
                  void onStatus(option.value);
                }}
              >
                {option.label}
              </button>
            ))}
          </div>
        </section>
      ) : busy === null && error === null ? (
        <PageStatus
          kind="empty"
          title="Nenhuma rifa carregada"
          detail="Crie uma rifa ou informe o slug para alterar o status."
        />
      ) : null}
    </>
  );
}

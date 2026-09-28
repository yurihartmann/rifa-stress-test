import type { FormEvent } from "react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { PageStatus } from "../components/PageStatus";
import { Field } from "../components/Field";
import { usePageTitle } from "../lib/usePageTitle";

export default function HomePage() {
  const navigate = useNavigate();
  const [slug, setSlug] = useState("");
  const [error, setError] = useState<string | null>(null);
  usePageTitle("Início");

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
      <h1>Abrir uma rifa</h1>
      {slug.trim().length === 0 && error === null ? (
        <PageStatus
          kind="empty"
          title="Nenhuma rifa selecionada"
          detail="Informe o slug público para abrir a vitrine. Exemplo: carro-ano-novo."
        />
      ) : null}
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
    </>
  );
}

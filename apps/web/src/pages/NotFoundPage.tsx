import { usePageTitle } from "../lib/usePageTitle";
import { PageStatus } from "../components/PageStatus";
import { Link } from "react-router-dom";

export default function NotFoundPage() {
  usePageTitle("Página não encontrada");
  return (
    <>
      <p className="eyebrow">Navegação</p>
      <h1>Página não encontrada</h1>
      <PageStatus
        kind="empty"
        title="Nada neste endereço"
        detail="A rota não existe. Volte ao início e abra uma rifa pelo slug."
      />
      <Link className="button" to="/">
        Ir para o início
      </Link>
    </>
  );
}

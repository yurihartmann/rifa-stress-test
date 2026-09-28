import { Suspense } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { apiBaseUrl } from "../api/client";
import { PageStatus } from "./PageStatus";

export function Layout() {
  return (
    <div className="shell">
      <header className="topbar">
        <NavLink to="/" className="brand">
          Rifa
        </NavLink>
        <nav className="nav" aria-label="Seções">
          <NavLink to="/">Início</NavLink>
          <NavLink to="/compras">Compras</NavLink>
          <NavLink to="/admin">Admin</NavLink>
        </nav>
      </header>
      <main id="conteudo">
        <Suspense
          fallback={
            <PageStatus kind="loading" title="Carregando" detail="Abrindo a página." />
          }
        >
          <Outlet />
        </Suspense>
      </main>
      <footer className="footer">
        <p>
          {apiBaseUrl().length > 0 ? (
            <>
              Laboratório local. O browser fala direto com a API em <code>{apiBaseUrl()}</code>.
            </>
          ) : (
            <>
              O browser fala com a API neste mesmo endereço (<code>/v1</code>).
            </>
          )}
        </p>
      </footer>
    </div>
  );
}

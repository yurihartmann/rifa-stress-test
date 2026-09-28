import { lazy } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";

const HomePage = lazy(() => import("./pages/HomePage"));
const RafflePage = lazy(() => import("./pages/RafflePage"));
const CheckoutPage = lazy(() => import("./pages/CheckoutPage"));
const PurchasesPage = lazy(() => import("./pages/PurchasesPage"));
const AdminPage = lazy(() => import("./pages/AdminPage"));
const NotFoundPage = lazy(() => import("./pages/NotFoundPage"));

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<HomePage />} />
          <Route path="rifa/:slug/checkout" element={<CheckoutPage />} />
          <Route path="rifa/:slug" element={<RafflePage />} />
          <Route path="compras" element={<PurchasesPage />} />
          <Route path="admin" element={<AdminPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

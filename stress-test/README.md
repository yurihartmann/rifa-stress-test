# Teste de carga k6

Cliente HTTP da API. Os scripts ficam neste diretório, usam só o contrato público e não importam o módulo Go nem acessam o Postgres.

## Smoke do funil

Com a API em `http://localhost:8080`, na raiz do repositório:

```bash
./stress-test/run.sh funnel
```

Esse comando exporta os defaults de laboratório e sobe um smoke curto: 2 VUs por 20 segundos, com parada graciosa de 30 segundos. O cenário `funnel` é o default. O resumo JSON fica em `stress-test/summary.json`.

O `setup` cria uma rifa (`POST /v1/admin/raffles`, 201) e abre (`PATCH /v1/admin/raffles/{id}` com `{"status":"open"}`, 200), com `total_tickets` 1000 e `ticket_price_cents` 1000. Se `K6_RAFFLE_SLUG` já estiver definido, o setup só lê essa rifa e exige `status=open`.

E-mail de cada iteração: `vu-{VU}-iter-{ITER}@stress.local`, já em minúsculas.

## Cenários

O primeiro argumento do `run.sh`, ou a variável `K6_SCENARIO`, escolhe o cenário:

| Cenário | Fluxo |
| --- | --- |
| `vitrine` | `GET /v1/raffles/{slug}` |
| `checkout` | `POST /v1/raffles/{slug}/orders` com `{email, quantity}` |
| `funnel` | vitrine, checkout, webhook `paid`, depois `GET /v1/purchases?email=` até `tickets.length === quantity` ou o timeout |
| `webhook-replay` | checkout e o mesmo `event_id` duas vezes; a segunda resposta tem de ser HTTP 200 |
| `over-capacity` | checkout com `available_tickets + 1`; espera HTTP 409 e `code=insufficient_tickets` |

`over-capacity` marca 409 como status esperado na métrica `http_req_failed`. Os outros cenários que compram consomem capacidade da rifa.

Os thresholds numéricos estão comentados em `script.js`. Ainda não há SLO medido.

## Aumentar a carga

```bash
K6_VUS=50 K6_DURATION=5m K6_GRACEFUL_STOP=1m K6_TOTAL_TICKETS=5000 ./stress-test/run.sh funnel
```

`K6_TOTAL_TICKETS` precisa cobrir VUs × iterações × `K6_QUANTITY` nos cenários que compram. O funil espera os bilhetes por `K6_POLL_TIMEOUT_MS` (15000). Suba esse valor junto com `K6_GRACEFUL_STOP`. O `ORDER_HOLD_TTL` da API precisa ser maior que essa espera, para o pedido não expirar no meio do cenário.

Outros exemplos:

```bash
./stress-test/run.sh vitrine
K6_VUS=20 K6_DURATION=2m ./stress-test/run.sh checkout
K6_RAFFLE_SLUG=minha-rifa ./stress-test/run.sh over-capacity
```

## Variáveis

| Variável | Default |
| --- | --- |
| `K6_BASE_URL` | `http://localhost:8080` |
| `K6_WEBHOOK_SECRET` | `lab-webhook-secret` |
| `K6_ADMIN_TOKEN` | `lab-admin-token` |
| `K6_SCENARIO` | `funnel` |
| `K6_RAFFLE_SLUG` | vazio (o setup cria a rifa) |
| `K6_TOTAL_TICKETS` | `1000` |
| `K6_TICKET_PRICE_CENTS` | `1000` |
| `K6_QUANTITY` | `1` |
| `K6_VUS` | `2` |
| `K6_DURATION` | `20s` |
| `K6_GRACEFUL_STOP` | `30s` |
| `K6_SETUP_TIMEOUT` | `120s` |
| `K6_HTTP_TIMEOUT` | `60s` |
| `K6_POLL_TIMEOUT_MS` | `15000` |
| `K6_POLL_INTERVAL_SEC` | `0.5` |

Os segredos vêm do ambiente. Os defaults coincidem com o Compose do laboratório. `K6_QUANTITY` não vale para `over-capacity`, que usa a capacidade restante mais um.

Chamar o k6 direto também funciona; os mesmos defaults estão em `lib/config.js`:

```bash
k6 run --summary-export=stress-test/summary.json stress-test/script.js
```

Se o binário `k6` não estiver no `PATH`, `./stress-test/run.sh` avisa e termina com código 127. Recusa de conexão significa que não há API escutando em `K6_BASE_URL`.

## Prometheus (opcional)

O smoke não envia métricas e não depende do Prometheus. Para ver a latência do cliente no Grafana, publique no remote write:

```bash
K6_PROMETHEUS_RW_SERVER_URL=http://localhost:9090/api/v1/write \
  ./stress-test/run.sh funnel -o experimental-prometheus-rw
```

## Contrato usado pelos checks

Checkout `201`:

```json
{
  "order_id": "uuid",
  "raffle_slug": "string",
  "email": "string",
  "quantity": 1,
  "amount_cents": 1000,
  "status": "pending_payment",
  "expires_at": "RFC3339",
  "payment": { "payment_id": "uuid", "qr_code_payload": "string" },
  "tickets": []
}
```

Vitrine `200`: `{id, slug, title, description, ticket_price_cents, total_tickets, status, available_tickets}`.

Compras `200`: `{email, purchases:[{order_id, raffle_slug, raffle_title, quantity, amount_cents, status, tickets:[{number}]}]}`. `number` é string. A emissão terminou quando `tickets.length === quantity`.

Webhook:

```json
{ "event_id": "string", "payment_id": "uuid", "status": "paid" }
```

Header `X-Webhook-Secret`. Fora do replay, `event_id` é único por pagamento (`vu-{VU}-iter-{ITER}-{payment_id}`). No replay, as duas chamadas da mesma iteração reutilizam `replay-vu-{VU}-iter-{ITER}`.

Erro: `{code, message}`. Neste cliente, 409 de capacidade usa `insufficient_tickets`. A API também pode responder `raffle_closed` e `order_expired`.

Criar rifa, admin, `Authorization: Bearer`:

```json
{
  "slug": "k6-...",
  "title": "Rifa de carga",
  "description": "Criada pelo setup do k6",
  "ticket_price_cents": 1000,
  "total_tickets": 1000
}
```

A resposta `201` precisa trazer `id`. O PATCH de abertura espera HTTP 200.

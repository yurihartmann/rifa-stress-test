# ADR-0009: Schema Postgres

## Status

Accepted

## Context

A reserva de capacidade, o pagamento simulado, a consulta por e-mail e a geração assíncrona de bilhetes precisam de um schema estável antes do código. A ADR-0004 descreve as transições. A ADR-0002 fixa GORM como acesso. Esta ADR é o desenho físico que os models e o `AutoMigrate` têm de produzir.

Não há soft delete. O model GORM não embute `gorm.Model`: o `id` é UUID gerado no use case, e não existe `deleted_at`.

## Decision

Banco único, schema `public`. Nomes de tabela e coluna em inglês, snake_case. Timestamps em `timestamptz`. Dinheiro em centavos (`bigint`).

O job de migração roda `AutoMigrate` e depois aplica os `CHECK` e os índices parciais desta ADR.

### `raffles`

| Coluna | Tipo | Regra |
| --- | --- | --- |
| `id` | `uuid` | PK |
| `slug` | `text` | único, não nulo |
| `title` | `text` | não nulo |
| `description` | `text` | não nulo, pode ser vazio |
| `ticket_price_cents` | `bigint` | não nulo, `> 0` |
| `total_tickets` | `integer` | não nulo, `> 0` |
| `reserved_tickets` | `integer` | não nulo, default `0` |
| `sold_tickets` | `integer` | não nulo, default `0` |
| `status` | `text` | não nulo, `draft`, `open` ou `closed` |
| `created_at` | `timestamptz` | não nulo |
| `updated_at` | `timestamptz` | não nulo |

```text
reserved_tickets >= 0
sold_tickets >= 0
reserved_tickets + sold_tickets <= total_tickets
```

Índice único em `slug`.

### `orders`

O preço fica copiado no pedido. Uma mudança posterior em `raffles.ticket_price_cents` não altera compra já aberta. `amount_cents = quantity * ticket_price_cents` no momento do checkout.

| Coluna | Tipo | Regra |
| --- | --- | --- |
| `id` | `uuid` | PK |
| `raffle_id` | `uuid` | FK para `raffles.id`, não nulo |
| `email` | `text` | não nulo, já normalizado |
| `quantity` | `integer` | não nulo, `> 0` |
| `amount_cents` | `bigint` | não nulo, `> 0` |
| `status` | `text` | não nulo, `pending_payment`, `paid` ou `expired` |
| `expires_at` | `timestamptz` | não nulo |
| `created_at` | `timestamptz` | não nulo |
| `updated_at` | `timestamptz` | não nulo |

Índices: `(email)`, `(status, expires_at)`, `(raffle_id)`.

### `payments`

Um pagamento por pedido.

| Coluna | Tipo | Regra |
| --- | --- | --- |
| `id` | `uuid` | PK |
| `order_id` | `uuid` | único, FK para `orders.id`, não nulo |
| `status` | `text` | não nulo, `pending` ou `paid` |
| `qr_code_payload` | `text` | não nulo |
| `paid_at` | `timestamptz` | nulo enquanto `pending` |
| `created_at` | `timestamptz` | não nulo |
| `updated_at` | `timestamptz` | não nulo |

### `payment_events`

Idempotência do webhook. A chave é o `event_id` enviado pelo cliente de teste.

| Coluna | Tipo | Regra |
| --- | --- | --- |
| `event_id` | `text` | PK |
| `payment_id` | `uuid` | FK para `payments.id`, não nulo |
| `created_at` | `timestamptz` | não nulo |

### `tickets`

Criada inteira na abertura da rifa: uma linha para cada número de `1` até `total_tickets`. `GenerateTicket` não insere linha; preenche `order_id` num sorteio entre as que ainda estão livres.

| Coluna | Tipo | Regra |
| --- | --- | --- |
| `id` | `uuid` | PK |
| `raffle_id` | `uuid` | FK para `raffles.id`, não nulo |
| `order_id` | `uuid` | FK para `orders.id`, nulo enquanto livre |
| `number` | `integer` | não nulo, de `1` até `total_tickets` da rifa |
| `assigned_at` | `timestamptz` | nulo enquanto `order_id` é nulo |
| `created_at` | `timestamptz` | não nulo |

Índice único `(raffle_id, number)`. Índice em `order_id`. Índice parcial `(raffle_id) WHERE order_id IS NULL` para o sorteio achar as linhas livres.

A representação `0001` não fica no banco. A API formata `number` com zeros à esquerda.

### `queue`

Fila de trabalhos no mesmo banco. A linha `generate_ticket` nasce no commit do webhook e diz ao worker para sortear os números daquele pedido. Não substitui `tickets` e não guarda o número sorteado.

| Coluna | Tipo | Regra |
| --- | --- | --- |
| `id` | `uuid` | PK |
| `type` | `text` | não nulo. Nesta etapa, `generate_ticket` |
| `aggregate_id` | `uuid` | não nulo. Para `generate_ticket`, é `orders.id` |
| `payload` | `jsonb` | não nulo. Inclui o contexto de trace |
| `status` | `text` | não nulo, `pending`, `processing`, `done` ou `failed` |
| `attempts` | `integer` | não nulo, default `0` |
| `available_at` | `timestamptz` | não nulo |
| `locked_at` | `timestamptz` | nulo fora de `processing` |
| `locked_by` | `text` | nulo fora de `processing` |
| `last_error` | `text` | nulo até a primeira falha |
| `created_at` | `timestamptz` | não nulo |
| `updated_at` | `timestamptz` | não nulo |

Índice em `(status, available_at)`. Índice único parcial em `aggregate_id` onde `type = 'generate_ticket'` e `status` em (`pending`, `processing`, `done`), para um pedido pago não ganhar dois trabalhos vivos ou concluídos. `failed` pode coexistir com uma reemissão manual futura, que esta etapa não faz.

```text
raffles ||--o{ orders
raffles ||--o{ tickets
orders  ||--|| payments
orders  ||--o{ tickets
payments ||--o{ payment_events
orders  ||--o| queue   (aggregate_id, type = generate_ticket)
```

## Consequences

- O model GORM e esta ADR são o contrato do banco. Uma coluna nova atualiza os dois no mesmo cambio.
- `CHECK` de capacidade pega escrita que fure o `UPDATE` condicional. Ele não substitui esse `UPDATE`: duas transações ainda competem pela linha de `raffles`.
- `payment_events` guarda todo `event_id` aceito, inclusive o replay. A tabela cresce com o k6. Para o laboratório isso é evidência de idempotência; a retenção fica para uma ADR futura se o volume incomodar.
- Consulta por e-mail usa o índice de `orders.email` e junta `tickets` por `order_id`. Bilhete com `order_id` nulo não aparece nessa consulta.
- Abrir uma rifa de N números insere N linhas em `tickets` na mesma transação da mudança de status.
- Teste de migração: banco vazio sobe com estas tabelas, checks e índices, e uma segunda execução do job não falha.

## Alternatives considered

- `gorm.Model` com `id` inteiro e soft delete. O contrato público já usa UUID, e bilhete vendido não deve sumir por delete lógico.
- Contador `next_number` no lugar do pool. Gera a sequência `1, 2, 3` em vez de sortear entre os números já criados.
- Queue em outra base. A confirmação do pagamento e a gravação do trabalho precisam commitar juntas.

# ADR-0004: Reserva de capacidade e emissão assíncrona

## Status

Accepted

## Context

O cliente entra pelo slug, escolhe uma quantidade de bilhetes e não escolhe números. O checkout pede só o e-mail e devolve um pagamento pendente. Os números nascem depois do webhook de pagamento, fora do request HTTP.

Sob carga, duas compras simultâneas não podem vender mais bilhetes do que `total_tickets`. Um retry do worker não pode emitir o mesmo pedido duas vezes. Uma compra abandonada precisa devolver a capacidade.

Não há broker de mensagens no escopo. Postgres já é a fonte de verdade.

## Decision

Uma rifa de 1000 números tem os bilhetes `1` a `1000` criados quando ela abre. O pedido não inventa número. `GenerateTicket` sorteia linhas ainda livres e grava o `order_id` nelas.

O número é `integer` no banco. Na API ele sai como string com zeros à esquerda, na largura de `total_tickets`. Com 1000 números, `1` aparece como `0001` e `1000` como `1000`.

A rifa também guarda `reserved_tickets` e `sold_tickets`, com invariante `reserved_tickets + sold_tickets <= total_tickets` e contadores não negativos. Esses contadores seguram a quantidade no checkout. As linhas de `tickets` são o estoque de números. `sold_tickets` sobe no pagamento, antes do sorteio; o `order_id` só entra no `GenerateTicket`.

Estados do pedido:

```text
pending_payment --ConfirmPayment--> paid
pending_payment --ExpireOrders----> expired
```

`paid` significa pagamento confirmado. Os números da rifa já existem, e podem ainda não estar ligados ao pedido. A resposta de leitura inclui só os bilhetes com `order_id` daquele pedido. A geração terminou quando essa lista tem `quantity` itens. Não há status extra no pedido para isso.

### Abertura da rifa

`UpdateRaffleStatus` para `open`, na transação do request de admin, insere os números `1..total_tickets` com `order_id` nulo. A segunda abertura não insere de novo. `total_tickets` fica imutável depois que o pool existe. Fechar a rifa não apaga números.

### Checkout (`OpenOrder`)

Na transação aberta pelo middleware HTTP (ADR-0002). Resposta 201 faz commit; 409 ou 404 faz rollback:

1. Atualizar a rifa só se `status = open` e `reserved_tickets + sold_tickets + quantity <= total_tickets`, somando `quantity` em `reserved_tickets`.
2. Se nenhuma linha atualizar, falhar com estoque insuficiente ou rifa fechada.
3. Inserir o pedido `pending_payment` com `expires_at = now + ORDER_HOLD_TTL` (padrão 15 minutos).
4. Inserir o pagamento `pending` e o payload do QR.

O request de checkout não publica evento e não escolhe números. A quantidade fica prometida no contador; as linhas continuam com `order_id` nulo.

### Confirmação (`ConfirmPayment`)

Na transação do request de webhook (ADR-0005). Resposta 200 faz commit; 4xx faz rollback:

1. Marcar o pagamento como `paid` somente se ainda estiver `pending`.
2. Marcar o pedido como `paid` somente se ainda estiver `pending_payment` e não expirado.
3. Mover a quantidade de `reserved_tickets` para `sold_tickets`.
4. Inserir um registro de queue `generate_ticket` com `aggregate_id = order_id`.

Pedido já `paid` com o mesmo evento é sucesso idempotente e não grava outro queue. Pedido `expired` não é reaberto pelo webhook.

### Expiração (`ExpireOrders`)

O worker busca pedidos `pending_payment` com `expires_at` no passado. Para cada um, numa transação própria no context, só expira se o status ainda for `pending_payment`, e só então devolve a quantidade a `reserved_tickets`. Nil do use case faz commit; erro faz rollback. A condição no `UPDATE` é o que impede a corrida com `ConfirmPayment`.

O perfil de stress pode aumentar `ORDER_HOLD_TTL` para o funil do k6 não expirar no meio do cenário.

### Geração (`GenerateTicket`)

O worker reivindica queue `pending` com `clause.Locking` `UPDATE SKIP LOCKED` e commita o claim (`processing` e incremento de `attempts`) antes de gerar. Em seguida abre outra transação no context e chama `GenerateTicket`:

1. Se o pedido já tem `quantity` bilhetes ligados, marca o queue como `done`.
2. Caso contrário, escolhe linhas da mesma rifa com `order_id` nulo, `ORDER BY random()`, `LIMIT` da quantidade que ainda falta, e `FOR UPDATE SKIP LOCKED`.
3. Grava `order_id` e `assigned_at` nessas linhas. Se o lock devolver menos linhas do que falta, repete o sorteio dentro da mesma transação. Se ainda faltar linha, o use case retorna erro.
4. Só um pedido `paid` recebe número.

Nil faz commit do vínculo junto com o queue `done`. Erro faz rollback dessa segunda transação, sem apagar o claim já commitado e sem deixar número pela metade. Uma terceira transação grava `last_error`. Se ainda há tentativa, volta o status para `pending` e reagenda `available_at`. Esgotadas as tentativas, marca `failed`.

Um claim `processing` com `locked_at` mais velho que o timeout do lock volta para `pending` sem incrementar `attempts` de novo. Isso cobre o worker que caiu depois do claim e antes do `done`.

O cliente não escolhe número no checkout. Dois pedidos pagos da mesma rifa concorrem pelas linhas livres; o `SKIP LOCKED` entrega conjuntos diferentes.

### Queue

`queue` é a fila de trabalhos dentro do Postgres. Não é um broker. No commit do webhook, a mesma transação que marca o pedido como `paid` insere uma linha `generate_ticket`. O worker lê essa linha e faz o sorteio. Se a API cair depois do commit, o trabalho continua na tabela.

O use case grava a linha. O worker é o adaptador que consome. Trocar o Postgres por um broker no futuro substitui esse adaptador, sem reescrever a regra de `ConfirmPayment`, desde que a publicação continue na mesma transação da venda.

Campos: `id`, `type`, `aggregate_id`, `payload`, `status` (`pending`, `processing`, `done`, `failed`), `attempts`, `available_at`, `locked_at`, `locked_by`. O desenho físico está na ADR-0009.

Esgotadas as tentativas, o status fica `failed` e a venda permanece `paid` para investigação. Não há devolução automática de capacidade nessa falha: a capacidade já foi vendida e o bilhete ainda é devido. As tabelas estão na ADR-0009.

## Consequences

- A linha da rifa é o ponto de serialização da capacidade no checkout. No stress test, lock e tempo desse `UPDATE` são métricas de primeira classe.
- O sorteio lê as linhas livres da rifa com `ORDER BY random()`. Com mil números o custo é baixo. Com um pool grande, essa ordenação vira o segundo ponto quente, já fora do request de checkout.
- A entrega do queue é at-least-once. A idempotência é `count(tickets.order_id = pedido) == quantity` mais o índice único `(raffle_id, number)`.
- O commit do pagamento não espera o número. A UI e o k6 observam a defasagem de propósito.
- Abrir uma rifa grande segura a transação do admin até o insert do pool terminar.
- Testes obrigatórios do use case: estoque insuficiente não cria pedido; webhook duplicado não move o contador duas vezes; expiração concorrente com pagamento só um dos dois vence; retry do worker não liga número extra; dois sorteios concorrentes não recebem o mesmo número.
- Teste de repositório obrigatório: duas transações concorrentes não ultrapassam `total_tickets`, e o pool aberto contém exatamente `1..total_tickets`.

## Alternatives considered

- Escolher números no checkout. O requisito pede quantidade. O sorteio fica depois do pagamento, quando o número passa a ser do pedido.
- Sequência `next_number`. Entrega `0001`, depois `0002`, e não o sorteio pedido.
- Sortear com `math/rand` no Go e atualizar pelo número calculado. Duas gerações podem escolher a mesma linha livre. O `SKIP LOCKED` no Postgres é quem separa os ganhadores.
- Emitir o vínculo dentro do request do webhook. Acopla a latência do sorteio à confirmação e perde o cenário assíncrono que o laboratório quer observar.
- Broker externo com publish depois do commit. Sem a linha de queue, uma falha entre o commit e o publish perde o sorteio ou exige outra varredura.

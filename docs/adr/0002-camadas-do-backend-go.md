# ADR-0002: Camadas do backend Go

## Status

Accepted

## Context

O backend é um serviço Go com HTTP, persistência Postgres, webhook e um worker. O fluxo de compra coordena reserva, pagamento e um efeito assíncrono. Essa receita precisa ser testável sem subir Gin nem Postgres.

O repositório ainda não tem código. Esta ADR fixa o corte de camadas antes da implementação, para o código novo já nascer nesse formato.

## Decision

Direção de dependência:

```text
transport -> application -> domain <- infrastructure
```

Layout de `apps/api`:

```text
apps/api/
├── cmd/
│   ├── api/
│   └── worker/
├── api/                         # OpenAPI gerado pelo Swag; não editar à mão
├── internal/
│   ├── app/
│   │   ├── service/
│   │   └── usecase/
│   ├── domain/
│   │   ├── dto/
│   │   ├── entity/
│   │   └── repository/
│   ├── infra/
│   │   ├── container/           # composition root
│   │   ├── database/            # *gorm.DB, transação no context, models
│   │   ├── repository/
│   │   └── setting/
│   └── interface/
│       ├── http/                # controllers e middleware de transação
│       └── worker/
├── Makefile
└── go.mod
```

| Camada | Responsabilidade |
| --- | --- |
| `cmd` | Subida, sinal de parada, timeout de shutdown, registro de rotas ou do loop do worker |
| `interface/http` | Bind, validação de payload, uma chamada de use case com `c.Request.Context()`, mapeamento HTTP. O middleware de transação abre e fecha o `*gorm.DB` |
| `interface/worker` | Claim do queue e uma chamada de use case. Cada unidade de trabalho abre e fecha a transação pelo mesmo helper de context |
| `app/usecase` | Ações nomeadas: abrir compra, confirmar pagamento, gerar bilhetes, expirar reserva, listar compras |
| `app/service` | Operação reutilizável pequena, só quando mais de um use case precisar dela |
| `domain` | Entidades, invariantes, erros sentinela, contratos de repositório. Sem import de GORM |
| `infra` | GORM, models, migração, settings tipados, cliente de telemetria |

O composition root em `internal/infra/container` é o único lugar que instancia repositório, use case e configuração concretos.

Repositórios expõem operações genéricas: `FindByID`, `FindOneByFilters`, `FindAllByFilters`, `Create`, `Update`. A seleção de negócio fica no use case, por filtro e ordenação. O webhook e a geração de bilhetes não viram métodos de repositório.

O acesso ao Postgres é GORM. Os models ficam em `internal/infra/database/model` e espelham a ADR-0009. O repositório traduz model e entidade de domínio. A atualização condicional de capacidade e o `FOR UPDATE SKIP LOCKED` usam `clause` do GORM; o use case decide quando chamar.

O schema sobe num job único do Compose, antes de `api` e `worker`, com `AutoMigrate` dos models e, em seguida, os `CHECK` e índices que tag GORM não expressa. `api` e `worker` não migram na subida.

Identificadores de código, tabelas, status e rotas ficam em inglês. Texto de produto no frontend fica em português.

HTTP usa Gin. `context.Context` é o primeiro argumento de toda operação que bloqueia ou faz I/O, e não é armazenado em struct. A transação GORM vive no context só durante o request ou a unidade de trabalho do worker. Erros de domínio são sentinela, para o transporte traduzir com `errors.Is`. Erros inesperados usam `%w`.

### Transação HTTP

Um middleware no grupo `/v1` é o dono do commit e do rollback. Healthcheck fica fora desse grupo.

1. `Begin` no `*gorm.DB` da aplicação.
2. Colocar essa transação no `context.Context` do request.
3. Chamar `c.Next()`.
4. Status final de 200 a 299: `Commit`. Qualquer outro status, ou panic: `Rollback`. O Gin inicia o status em 200, então o handler de erro precisa gravar 4xx ou 5xx antes de retornar.
5. O body da resposta só segue para o cliente depois do `Commit`. Se o `Commit` falha, o cliente recebe 500 e a transação sofre `Rollback`.

Handler, use case e repositório não chamam `Begin`, `Commit` nem `Rollback`. O repositório obtém o `*gorm.DB` com `database.FromContext(ctx)`. Context sem transação é erro de programação, sem fallback para a conexão global.

O mapeamento de erro do handler grava o status HTTP antes de retornar. 409, 404, 401 e 400 desfazem as escritas daquele request.

### Transação do worker

O worker não tem status HTTP. O runner usa o mesmo context e a mesma regra de resultado: `Commit` quando o use case retorna nil, `Rollback` quando retorna erro. O registro durável de tentativa do queue, quando a geração falha, acontece numa transação seguinte, já descrita na ADR-0004.

Use cases da primeira etapa:

| Use case | Coordenação |
| --- | --- |
| `OpenOrder` | Validar rifa aberta, reservar capacidade, gravar pedido e pagamento pendente |
| `ConfirmPayment` | Aplicar webhook idempotente, mover reserva para venda, gravar queue na transação do request |
| `GenerateTicket` | Sortear bilhetes já criados e gravar o `order_id`, sem duplicar em retry |
| `ExpireOrders` | Liberar reserva de pedidos pendentes vencidos |
| `ListPurchasesByEmail` | Ler pedidos e bilhetes daquele e-mail |
| `CreateRaffle` / `UpdateRaffleStatus` | Backoffice. Ao abrir, criar o pool `1..total_tickets` |

## Consequences

- Teste de use case usa fake de repositório e prova a ordem reserva, persistência, queue, sem I/O.
- Teste de repositório cobre o `UPDATE` condicional de capacidade contra Postgres, via GORM.
- Teste do middleware: 2xx dá commit, status fora de 2xx e panic dão rollback, e falha de commit responde 500.
- Teste de rota cobre o contrato HTTP, não a regra de estoque.
- Cada request em `/v1` segura uma conexão do pool até o commit ou o rollback. O pool precisa cobrir os requests em voo do stress test.
- Trocar Gin não reescreve a regra de compra. Trocar GORM reescreve repositório, model e middleware.
- A primeira entrega tem mais pastas do que um CRUD mínimo. O fluxo já cruza transação, webhook e worker, então o corte se paga agora.

## Alternatives considered

- Controller chamando repositório direto. O webhook, a reserva e a geração deixam de ter um dono único da transição de estado.
- Clean architecture com um módulo por entidade e ports para cada verbo. O serviço tem um agregado principal e não justifica essa malha.
- pgx com SQL explícito. Deixa o `UPDATE` condicional e o `SKIP LOCKED` visíveis no texto da query. GORM é a escolha para models, middleware e uma transação só no context.
- Commit e rollback dentro do use case. Espalha o fechamento da transação e impede o middleware de decidir pelo status HTTP.

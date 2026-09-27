# ADR-0008: Contrato HTTP, Swagger e k6

## Status

Accepted

## Context

Frontend, k6 e webhook precisam do mesmo contrato. A documentação HTTP tem de nascer do código dos handlers e ser publicada no laboratório. O teste de carga é um cliente de fora do processo, repetindo o funil real, não um teste interno com repositório falso.

## Decision

A API HTTP é JSON, Gin, prefixo `/v1`. Swag gera a especificação a partir das anotações dos handlers. A saída versionada fica em `apps/api/api/` (`docs.go`, `swagger.json`, `swagger.yaml`). Esses arquivos não são editados à mão. O alvo de geração segue o padrão do serviço: `swag fmt` e `swag init` sobre `cmd/api` e os pacotes de handler, DTO e entidade expostos no contrato. A UI fica em `/v1/docs`.

O base path documentado é o base path servido.

Rotas da primeira etapa:

| Método e rota | Uso |
| --- | --- |
| `GET /v1/raffles/{slug}` | Vitrine |
| `POST /v1/raffles/{slug}/orders` | Checkout: e-mail e quantidade |
| `POST /v1/payments/webhooks` | Confirmação simulada |
| `GET /v1/purchases?email=` | Compras e bilhetes |
| `POST /v1/admin/raffles` | Criar rifa |
| `PATCH /v1/admin/raffles/{id}` | Alterar status |

Códigos:

| Situação | Código |
| --- | --- |
| Pedido criado | 201 |
| Leitura e webhook idempotente já aplicado | 200 |
| Corpo inválido | 400 |
| Admin ou webhook sem credencial | 401 |
| Slug ou pagamento inexistente | 404 |
| Sem estoque, rifa fechada ou pedido expirado | 409 |
| Falha inesperada | 500 |

Erros usam um único formato JSON do pacote de erro do serviço. DTOs de request e response são tipos de borda. A entidade de persistência não é o schema público quando carrega contador interno ou segredo.

Esquemas de segurança no OpenAPI: bearer estático do admin e header `X-Webhook-Secret`. Rotas públicas não declaram segurança.

k6 vive em `stress-test/` e usa só o contrato público. Não importa o módulo Go e não acessa o Postgres. Cenários iniciais, cada um com limiar próprio quando os números forem definidos:

1. leitura da vitrine por slug
2. checkout
3. funil completo: vitrine, checkout, webhook, consulta até os bilhetes aparecerem
4. webhook repetido do mesmo `event_id`
5. checkout acima da capacidade restante

Dados de rifa do teste vêm de um setup explícito (backoffice ou seed), com `total_tickets` conhecido. E-mails do cenário são determinísticos por VU e iteração.

O resultado do k6 vai para o Prometheus (ADR-0007) e para um resumo local em `stress-test/`.

## Consequences

- Mudança de rota ou campo exige atualizar a anotação e regenerar `apps/api/api/` no mesmo cambio.
- O frontend e o k6 podem ser escritos contra o `swagger.json`, sem ler o use case.
- 409 cobre conflitos de negócio diferentes. O corpo do erro discrimina o caso; o cliente de carga não depende do texto livre.
- k6 contra a API real é mais lento que teste de use case. Os dois coexistem: use case prova a regra, k6 prova o funil sob concorrência.
- Ainda não há número de VU, duração nem threshold. Esta ADR fixa o lugar e os cenários; a meta numérica entra quando houver um baseline medido.

## Alternatives considered

- OpenAPI escrito à mão, separado do Gin. Desvia do fluxo Swag já escolhido para o serviço Go e tende a divergir do handler.
- k6 dentro de `go test` ou apontando para funções internas. Deixa de medir o processo, o pool de conexões e o webhook como um cliente real.
- Gerador de cliente TypeScript obrigatório na primeira etapa. O SPA é pequeno e pode consumir os poucos endpoints com tipos locais até o contrato estabilizar.

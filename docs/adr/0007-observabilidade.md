# ADR-0007: Observabilidade

## Status

Accepted

## Context

O objetivo do repositório é stress test com sinais para explicar saturação, erro e latência. A stack pedida inclui Grafana e Loki para logs e traces.

Loki armazena logs. Traces precisam de um backend de trace para a Grafana correlacionar latência de request, tempo de transação e atraso do worker. Métricas de capacidade (RPS, p95, saturação do Postgres, profundidade do queue) não cabem em Loki.

API e worker são processos diferentes. Um pedido pago e a emissão dos bilhetes precisam ser o mesmo rastro.

## Decision

Três sinais, uma interface:

| Sinal | Produção | Armazenamento | Consulta |
| --- | --- | --- | --- |
| Log | JSON em stdout, com `trace_id` e `span_id` | Loki, via Grafana Alloy | Grafana |
| Trace | OTLP a partir do OpenTelemetry SDK | Tempo, via OpenTelemetry Collector | Grafana |
| Métrica | OTLP a partir do mesmo SDK, mais métricas do k6 | Prometheus | Grafana |

O aplicativo não abre conexão com Loki, Tempo ou Prometheus. Ele escreve log no stdout e exporta OTLP para o Collector. O Collector exporta traces ao Tempo e métricas ao Prometheus. O Alloy lê o stdout dos containers `api` e `worker` e envia ao Loki.

Propagação: o HTTP de entrada extrai o contexto W3C. O use case recebe o `context.Context` do request ou do claim do worker. O worker inicia um span consumidor ligado ao trace gravado na mensagem de queue, para o pagamento e a emissão aparecerem no mesmo trace.

Spans mínimos: request HTTP, transação de checkout, transação de confirmação, claim do queue, `GenerateTicket`, query de capacidade. Atributos: `raffle_id`, `order_id`, `payment_id`, `quantity`. Sem e-mail e sem segredo. O span da transação inclui se o fechamento foi commit ou rollback.

Métricas mínimas da aplicação:

- duração e contagem HTTP por rota, método e status
- duração da transação de reserva
- pedidos por status
- queue por status e idade do mais antigo `pending`
- bilhetes emitidos e falhas de emissão

O k6 publica as métricas de cliente (latência vista pelo gerador de carga) no Prometheus, para o dashboard comparar tempo no cliente e tempo no servidor.

Dashboards versionados em `deploy/grafana`:

- funil: checkout, webhook, emissão
- Postgres e ponto quente da reserva
- queue: profundidade, idade, falha
- logs com salto para o trace pelo `trace_id`

Healthchecks de `api` e `worker` não entram nesses dashboards como sinal de negócio.

## Consequences

- Sobe Collector, Alloy, Loki, Tempo e Prometheus além do que o nome "Loki" sozinho sugere. Sem Tempo, a Grafana não tem trace para correlacionar com a linha de log.
- O volume de log em um stress test pode dominar disco local. O Compose deve limitar retenção de Loki, Tempo e Prometheus em valores curtos de laboratório.
- Instrumentação no use case e no repositório da reserva é parte da definição de pronto, não um passo posterior.
- Teste de fumaça da stack: um checkout gera uma linha de log com `trace_id` que abre um trace com o span da reserva.

## Alternatives considered

- Guardar traces no Loki. O Loki não é backend de traces; a correlação pedida ficaria sem serviço de consulta de spans.
- Cliente Go empurrando log direto ao Loki e métrica direto ao Prometheus. Acopla o processo de negócio aos backends e cria outro caminho de falha durante o teste de carga.
- Jaeger no lugar do Tempo. Funciona como backend de trace. Tempo permanece porque o desenho alvo é a suíte Grafana já pedida, com descoberta de trace a partir do log.

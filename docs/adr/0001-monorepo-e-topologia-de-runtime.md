# ADR-0001: Monorepo e topologia de runtime

## Status

Accepted

## Context

O laboratório precisa de API, worker assíncrono, frontend com backoffice, Postgres, stack de observabilidade e testes de carga no mesmo repositório. Docker Compose é o ambiente local único. k6 conversa com a API como um cliente externo. Não há segundo serviço de negócio nesta etapa.

O fluxo de compra confirma o pagamento na API e só depois emite os bilhetes. Esse efeito não pode depender do processo HTTP continuar vivo depois da resposta.

## Decision

Um monorepo, orquestrado pelo Makefile da raiz e por um único `docker-compose`. Sem ferramenta de monorepo JavaScript.

```text
/
├── apps/
│   ├── api/                 # módulo Go: processo HTTP e processo worker
│   └── web/                 # SPA React compilada para arquivos estáticos
├── deploy/
│   ├── docker-compose.yml
│   ├── grafana/
│   ├── loki/
│   ├── tempo/
│   ├── prometheus/
│   ├── otel-collector/
│   └── alloy/
├── stress-test/
├── docs/
│   └── adr/
└── Makefile
```

O módulo Go fica em `apps/api`. O path do `go.mod` é `github.com/yurihartmann/rifa-stress-test/apps/api` até o remote publicado exigir outro.

Processos de aplicação:

| Processo | Responsabilidade |
| --- | --- |
| `api` | HTTP público, backoffice e webhook de pagamento |
| `worker` | Emissão de bilhetes e expiração de reservas |
| `web` | Nginx servindo o build estático do SPA |

`api` e `worker` são dois binários do mesmo módulo e da mesma imagem, com comandos diferentes. Os dois compartilham o Postgres. Nenhum deles importa o outro em runtime.

A imagem Go é multi-stage e contém só o binário em execução. A imagem web contém só o build estático e o Nginx.

Limites de container no Compose, em `deploy.resources.limits`:

| Serviço | CPU | Memória |
| --- | --- | --- |
| `api`, `worker` e o job de migração | `1` | `512M` |
| `postgres` | `1` | `2G` |

`api` e `worker` recebem o limite cada um. `GOMAXPROCS=1` e `GOMEMLIMIT=450MiB` acompanham o cgroup, para o runtime usar um núcleo e coletar memória antes do limite de 512 MB. Os parâmetros de memória do Postgres cabem dentro de 2 GB. Web e a stack de observabilidade ficam sem esses limites.

O Compose sobe também Postgres, job único de migração com GORM `AutoMigrate`, Grafana, Loki, Tempo, Prometheus, OpenTelemetry Collector e Grafana Alloy. A topologia de telemetria está na ADR-0007. O schema físico está na ADR-0009. Os scripts de carga ficam em `stress-test/`.

Configuração entra por variáveis de ambiente validadas na subida de cada processo. Segredos de laboratório (`PAYMENT_WEBHOOK_SECRET`, `ADMIN_TOKEN`) não entram na imagem.

## Consequences

- O funil de stress tem um alvo HTTP estável. k6 não entra na rede interna do worker.
- A emissão de bilhetes sobrevive ao término do request que confirmou o pagamento.
- Dois processos exigem healthcheck e desligamento gracioso separados.
- Um único Postgres concentra dados de negócio e a fila transacional. A saturação desse banco, presa a 1 CPU e 2 GB, é um resultado esperado do laboratório.
- A API e o worker podem ser mortos pelo limite de 512 MB. Esse OOM entra nas métricas do stress test.
- Juntos, os dois processos Go podem usar 2 CPUs e 1 GB. O Postgres usa o próprio teto, à parte.
- Não há Nx, Turborepo nem workspaces npm além do `apps/web`.

## Alternatives considered

- Vários repositórios. Aumenta o custo de subir o laboratório e de versionar o contrato que o k6 e o frontend consomem juntos.
- Um único processo Go com goroutines para o worker. Mistura o ciclo de vida HTTP com o consumidor e dificulta observar fila e API como serviços distintos no Compose.
- Broker externo (NATS, Redis ou RabbitMQ) já na topologia inicial. O requisito não pede broker, e a ADR-0004 resolve a entrega assíncrona com queue no Postgres.

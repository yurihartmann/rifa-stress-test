# rifa-stress-test
Monorepo for study performance and observability

## Como subir

Na raiz do repositório:

```bash
make up
```

Sobe Postgres, a migração, a API, o worker, o frontend e a observabilidade definidos em `deploy/docker-compose.yml`.

| Serviço | URL |
| --- | --- |
| API | http://localhost:8080 |
| Web | http://localhost:8081 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |

Grafana abre como admin anônimo. O login `admin` / `admin` também funciona. Segredos de laboratório: `PAYMENT_WEBHOOK_SECRET=lab-webhook-secret` e `ADMIN_TOKEN=lab-admin-token`. Postgres é `rifa` / `rifa` / `rifa`.

`make logs` e `make ps` acompanham os containers. `make down` para a stack e mantém o volume do Postgres. `make test-api` roda `go test ./...` em `apps/api` quando o módulo existe.

O k6 roda no host, contra http://localhost:8080, e pode enviar métricas para http://localhost:9090/api/v1/write.

Para o mesmo stack no Dokploy, use `deploy/docker-compose.dokploy.yml`. O passo a passo está em `deploy/DOKPLOY.md`.

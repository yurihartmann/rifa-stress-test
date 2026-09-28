# Deploy no Dokploy

Stack de laboratório (Postgres, migrate, API, worker, web e observabilidade) como aplicação Compose. O `make up` local continua em `deploy/docker-compose.yml` e não usa este arquivo.

## Aplicação

| Campo | Valor |
| --- | --- |
| Projeto | `rifa` (já criado) |
| Tipo | Compose, sourceType `github` |
| composePath | `./deploy/docker-compose.dokploy.yml` |
| Projeto Compose (`-p`) | `rifa` |

O Dokploy, ao criar a aplicação, grava `appName` com um sufixo aleatório (por exemplo `rifa-08mfjd`). O comando padrão usa esse nome em `-p` e, com o `.env` ligado, também grava `COMPOSE_PROJECT_NAME` com o mesmo sufixo. Os dois vencem o `name: rifa` do arquivo e viram o label `com.docker.compose.project`. O Alloy só mantém logs quando esse label é exatamente `rifa`.

A produção não muda o regex do Alloy. Ela substitui o comando de deploy por inteiro (o Dokploy prefixa `docker` e não acrescenta flags ao que você escreve). O `-p rifa` tem precedência sobre o `COMPOSE_PROJECT_NAME` do `.env`. O `--env-file` continua obrigatório: o deploy roda com `env -i` e, sem esse arquivo, os segredos não interpolam.

```text
compose -p rifa --env-file ./deploy/.env -f ./deploy/docker-compose.dokploy.yml up -d --build --remove-orphans
```

Se o comando padrão desta aplicação tiver flags a mais (`--pull always`, `--project-directory`), copie-as e troque só o `-p` para `rifa`. O campo é o Command documentado em [Docker Compose](https://docs.dokploy.com/docs/core/docker-compose).

`autoDeploy` é opcional.

## Caminhos e ambiente

`build.context` e os volumes (`./alloy`, `./grafana`, `./loki`, `./tempo`, `./prometheus`, `./otel-collector`) são relativos a este arquivo. O Dokploy entra na raiz do clone e roda `docker compose -f ./deploy/docker-compose.dokploy.yml` sem `--project-directory`. Nesse modo o Compose resolve esses caminhos a partir de `deploy/`.

Não cadastre mounts extras na aplicação Compose. Com mounts, o Dokploy passa `--project-directory` na raiz do repositório e `../apps/api` deixa de apontar para o código.

Deixe ligada a criação do arquivo `.env` (padrão, `createEnvFile`). O Dokploy grava `./deploy/.env`. O deploy roda com ambiente limpo (`env -i`) e só interpola esse arquivo quando o comando leva `--env-file ./deploy/.env`.

## isolatedDeployment

Deixe `isolatedDeployment=false`.

O Alloy monta `/var/run/docker.sock`. Com isolamento ligado, o Dokploy também reescreve os nomes dos serviços. O Nginx do `web`, o Grafana e o Alloy usam os hostnames internos `api`, `loki`, `prometheus` e `tempo`. Esses nomes precisam permanecer os do arquivo.

Não coloque labels Traefik nem a rede `dokploy-network` neste arquivo. O painel grava o domínio no serviço e injeta a rota.

## Variáveis

Defina na aplicação Compose (o Dokploy interpola o arquivo; não há segredo padrão neste compose). Senha do Postgres só com letras e números: ela entra na `DATABASE_URL`.

Obrigatórias:

- `POSTGRES_PASSWORD`
- `PAYMENT_WEBHOOK_SECRET`
- `ADMIN_TOKEN`
- `GF_SECURITY_ADMIN_PASSWORD`

Opcionais (o padrão está entre parênteses):

- `POSTGRES_USER` (`rifa`)
- `POSTGRES_DB` (`rifa`)
- `GF_SECURITY_ADMIN_USER` (`admin`)
- `GF_AUTH_ANONYMOUS_ENABLED` (`false`)
- `VITE_API_BASE_URL` (`/`)

`VITE_API_BASE_URL` é argumento de build. Mudou o valor, faça rebuild da imagem `web`. Os segredos de laboratório do compose local não servem neste host público.

## Domínios

O Traefik usa o nome do serviço e a porta interna do container. Nada é publicado na porta do host.

| Uso | serviceName | porta |
| --- | --- | --- |
| UI | `web` | `80` |
| API HTTP (k6 e acesso direto) | `api` | `8080` |
| Grafana | `grafana` | `3000` |
| Prometheus | `prometheus` | `9090` |

Publique `web`, `api` e `grafana`. Deixe o Prometheus sem domínio: o Grafana fala com ele pela rede do Compose. Só crie domínio em `prometheus:9090` se o k6 no seu computador precisar do remote write.

`api` e `worker` não definem healthcheck. A imagem Go é distroless/static e não tem shell, `wget` nem `curl`; um check que falha deixa o container unhealthy e o Traefik tira a API do ar.

Postgres, migrate, worker, otel-collector, tempo, loki e alloy ficam só na rede interna.

## Browser e API

A imagem `web` é Nginx. Com `VITE_API_BASE_URL=/` (padrão deste arquivo), o browser chama `/v1/...` no mesmo host da UI. O Nginx encaminha `/v1/` para `http://api:8080`. O certificado termina no Traefik; o browser não usa `localhost`.

Para o browser falar direto com o domínio público da API, defina `VITE_API_BASE_URL` com essa URL absoluta (incluindo `https://`) e faça rebuild. A API já devolve CORS para o `Origin` recebido.

O `make up` local continua gravando `http://localhost:8080` na imagem e publicando as portas do laptop. O proxy `/v1` existe na imagem, mas o browser local não passa por ele.

## k6

Roda na sua máquina, contra a URL pública da API. `K6_WEBHOOK_SECRET` e `K6_ADMIN_TOKEN` têm de ser os mesmos valores de `PAYMENT_WEBHOOK_SECRET` e `ADMIN_TOKEN`:

```bash
K6_BASE_URL=https://<domínio-da-api> \
  K6_WEBHOOK_SECRET="$PAYMENT_WEBHOOK_SECRET" \
  K6_ADMIN_TOKEN="$ADMIN_TOKEN" \
  ./stress-test/run.sh funnel
```

O remote write do laboratório aponta para `localhost:9090`. Sem domínio no Prometheus, não use esse caminho neste deploy.

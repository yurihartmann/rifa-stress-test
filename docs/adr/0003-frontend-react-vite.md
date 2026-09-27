# ADR-0003: Frontend com React e Vite

## Status

Accepted

## Context

A primeira etapa pede um frontend pequeno com vitrine da rifa, checkout e backoffice. O alvo de stress é a API. O browser é o cliente de demonstração e de conferência manual do mesmo contrato que o k6 executa.

A vitrine usa o slug público `/rifa/carro-ano-novo`. O checkout coleta só o e-mail e mostra um QR code. O backoffice cria e abre rifas. As compras são consultadas por e-mail.

## Decision

O frontend é um SPA em `apps/web`, com React, TypeScript, Vite e React Router. O build gera arquivos estáticos. O Compose publica esses arquivos com Nginx.

Não há BFF e não há servidor Node em runtime. O browser chama a API Go documentada na ADR-0008. A base da API entra em `VITE_API_BASE_URL` no build da imagem.

Rotas:

| Rota | Função |
| --- | --- |
| `/rifa/:slug` | Dados da rifa e escolha da quantidade de bilhetes |
| `/rifa/:slug/checkout` | E-mail, criação do pedido e QR code |
| `/compras` | Consulta por e-mail de pedidos e bilhetes |
| `/admin` | Criar rifa e alterar o status. Envia `Authorization: Bearer` com o token de laboratório guardado em `sessionStorage` |

O QR code é desenhado no browser a partir do payload textual devolvido pela API. A API não gera imagem.

Depois do pagamento, a tela de checkout consulta o pedido até `tickets.length` igualar `quantity`. A emissão continua assíncrona no worker.

O backoffice mora no mesmo SPA. Não há segundo app frontend.

## Consequences

- O processo `web` não participa do caminho de carga. k6 fala só com a API.
- A imagem de produção do frontend não precisa de Node.
- SEO e renderização no servidor ficam fora desta etapa. A página da rifa depende de JavaScript.
- O token de admin no `sessionStorage` é aceitável no laboratório local e é frágil em qualquer exposição pública. A ADR-0006 limita esse token ao ambiente de teste.
- Bibliotecas de UI e de data fetching não fazem parte desta decisão. A primeira implementação pode usar `fetch` e CSS próprio.

## Alternatives considered

- Next.js. Traz servidor Node e renderização no servidor sem mudar o alvo do k6, que é o contrato HTTP da API.
- Templates HTML no processo Go. Reduz um deploy, e o checkout com QR, a espera dos bilhetes e o backoffice passam a evoluir dentro do binário da API.
- Vue ou Svelte com Vite. Atendem o mesmo desenho de SPA estático. React é a escolha pelo ecossistema de formulário, rota e componentes de backoffice num app que deve continuar pequeno.

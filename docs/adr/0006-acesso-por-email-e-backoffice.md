# ADR-0006: Acesso por e-mail e backoffice

## Status

Accepted

## Context

O cliente acompanha compras e bilhetes sem criar conta, usando apenas o e-mail informado no checkout. O backoffice cria rifas e muda o status delas. Os dois acessos têm riscos diferentes: o primeiro é uma consulta pública por um identificador fraco; o segundo altera estoque e catálogo.

O laboratório roda em Docker local. Esta decisão não autoriza exposição pública desse modelo.

## Decision

O e-mail é normalizado no transporte: trim e minúsculas. Ele é atributo do pedido, não uma conta, senha ou sessão.

```text
GET /v1/purchases?email=cliente@example.com
```

A resposta lista somente pedidos daquele e-mail, com os bilhetes de cada um. E-mail sem compras responde 200 e lista vazia. Não há endpoint para enumerar e-mails existentes.

Não existe cadastro, login de cliente nem token de comprador nesta etapa.

O backoffice usa rotas `/v1/admin/...` com `Authorization: Bearer <ADMIN_TOKEN>`. O token é um segredo estático de ambiente, comparado em tempo constante. Ele autoriza criar rifa e alterar status. Não autoriza o webhook de pagamento. O webhook continua com `X-Webhook-Secret`.

Status de rifa: `draft`, `open`, `closed`. A passagem para `open` cria o pool de números da ADR-0004. Só `open` aceita `OpenOrder`. Fechar uma rifa não apaga pedidos `pending_payment` nem os números; a expiração ou o pagamento seguem as regras da ADR-0004. Nova reserva deixa de ser aceita.

Logs de aplicação registram `raffle_id`, `order_id`, `payment_id` e `trace_id`. O e-mail completo não entra em log de info.

## Consequences

- Quem sabe o e-mail lê os bilhetes daquele e-mail. No laboratório isso é o requisito funcional. Numa evolução pública, esta ADR deve ser substituída por um link de acesso de uso único.
- k6 precisa controlar os e-mails que gera, para a consulta não varrer a base inteira e para o cenário ser reproduzível.
- O token de admin no frontend (ADR-0003) é o mesmo segredo. Vazar o bundle ou o Compose vaza o backoffice.
- Testes: normalização de e-mail, lista vazia, isolamento entre dois e-mails, admin sem token, admin não confirma pagamento.

## Alternatives considered

- Conta com senha ou magic link já na primeira etapa. Atende melhor um produto público e adiciona fluxo que o requisito explícito não pede.
- Cookie de sessão criado no checkout. Amarra o browser que comprou e quebra a consulta "apenas com e-mail" noutro dispositivo, que é o requisito.
- Backoffice sem autenticação por estar em localhost. O Compose ainda publica a porta, e o Swagger ficaria com rotas de escrita abertas.

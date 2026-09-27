# ADR-0005: Pagamento simulado e webhook

## Status

Accepted

## Context

O checkout precisa entregar um QR code de pagamento, e um webhook posterior confirma esse pagamento. Nesta etapa o provedor é falso. O k6 e o botão de demonstração do frontend precisam conseguir confirmar um pagamento sem integração bancária.

O webhook é uma borda pública do laboratório. Retry do cliente de teste é esperado. A confirmação muda estoque e dispara emissão de bilhete, então repetir o POST não pode vender de novo.

## Decision

Não existe serviço separado de pagamento. O use case `OpenOrder` grava o pagamento e devolve o contrato:

```json
{
  "payment_id": "uuid",
  "qr_code_payload": "string"
}
```

`qr_code_payload` é uma string opaca estável, suficiente para o browser desenhar o QR. Não é imagem, não é PIX real e não chama rede externa.

Confirmação:

```text
POST /v1/payments/webhooks
X-Webhook-Secret: <PAYMENT_WEBHOOK_SECRET>
```

```json
{
  "event_id": "string",
  "payment_id": "uuid",
  "status": "paid"
}
```

Regras da borda:

- O segredo ausente ou diferente responde 401. O middleware de transação faz rollback e nenhuma escrita de negócio permanece.
- `status` diferente de `paid` responde 400. Nesta etapa não há estorno nem pagamento parcial.
- `event_id` é único. A primeira aplicação confirma. A repetição do mesmo `event_id` responde 200 sem alterar contadores nem queue.
- Outro `event_id` para um pagamento já `paid` responde 200 e não reaplica a venda.
- Pedido `expired` ou rifa sem reserva compatível responde 409.
- Pagamento desconhecido responde 404.

O controller valida o corpo e o segredo e chama só `ConfirmPayment`. Commit e rollback ficam com o middleware da ADR-0002, conforme o status HTTP. As escritas dessa confirmação estão na ADR-0004.

O frontend de demonstração pode chamar esse webhook com o segredo de laboratório para simular o banco. O k6 usa o mesmo endpoint. Não há endpoint paralelo "marcar como pago" com outra regra.

Timeout e leitura do body ficam no servidor HTTP. Não há cliente HTTP saindo da API para um banco.

## Consequences

- O cenário de carga inclui um POST de webhook autenticado por segredo compartilhado. O segredo vive no ambiente do k6 e do Compose, não no repositório.
- Qualquer pessoa com o segredo confirma pagamentos. Isso é aceitável no laboratório isolado e insuficiente para produção.
- O contrato do webhook entra no Swagger com esquema de segurança próprio, separado do token de admin.
- Testes de rota: segredo inválido, replay do mesmo `event_id`, segundo evento para pagamento já pago, pedido expirado.

## Alternatives considered

- Provedor sandbox externo. Acrescenta rede, credencial e indisponibilidade fora do que o stress test quer controlar.
- Endpoint autenticado como admin para confirmar pagamento, com o webhook ficando para depois. Criaria duas regras de confirmação.
- Gerar PNG do QR na API. Gasta CPU no checkout, e o cliente de carga não precisa da imagem.

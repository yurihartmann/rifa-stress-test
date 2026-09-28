export function raffleChecks(body, slug) {
  return {
    'vitrine status 200': function (res) {
      return res.status === 200;
    },
    'raffle id': function () {
      return body && typeof body.id === 'string' && body.id.length > 0;
    },
    'raffle slug': function () {
      return body && body.slug === slug;
    },
    'raffle title': function () {
      return body && typeof body.title === 'string';
    },
    'raffle description': function () {
      return body && typeof body.description === 'string';
    },
    'ticket_price_cents': function () {
      return body && typeof body.ticket_price_cents === 'number';
    },
    'total_tickets': function () {
      return body && typeof body.total_tickets === 'number';
    },
    'raffle status': function () {
      return body && typeof body.status === 'string';
    },
    'available_tickets': function () {
      return body && typeof body.available_tickets === 'number';
    },
  };
}

export function orderChecks(body, email, quantity, priceCents) {
  return {
    'checkout status 201': function (res) {
      return res.status === 201;
    },
    'order_id': function () {
      return body && typeof body.order_id === 'string' && body.order_id.length > 0;
    },
    'order raffle_slug': function () {
      return body && typeof body.raffle_slug === 'string';
    },
    'order email': function () {
      return body && body.email === email;
    },
    'order quantity': function () {
      return body && body.quantity === quantity;
    },
    'order amount_cents': function () {
      return body && body.amount_cents === quantity * priceCents;
    },
    'order status pending_payment': function () {
      return body && body.status === 'pending_payment';
    },
    'order expires_at': function () {
      return body && typeof body.expires_at === 'string' && body.expires_at.length > 0;
    },
    'payment_id': function () {
      return body && body.payment && typeof body.payment.payment_id === 'string' && body.payment.payment_id.length > 0;
    },
    'qr_code_payload': function () {
      return body && body.payment && typeof body.payment.qr_code_payload === 'string';
    },
    'checkout tickets array': function () {
      return body && Array.isArray(body.tickets);
    },
  };
}

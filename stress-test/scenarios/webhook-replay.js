import { check } from 'k6';
import { config } from '../lib/config.js';
import { orderChecks } from '../lib/contract.js';
import { createOrder, postWebhook } from '../lib/http.js';
import { replayEventId, stressEmail } from '../lib/ids.js';
import { readJson } from '../lib/json.js';

export function webhookReplay(data) {
  const email = stressEmail();
  const orderRes = createOrder(data.slug, email, config.quantity);
  const order = readJson(orderRes);
  const orderOk = check(orderRes, orderChecks(order, email, config.quantity, data.ticketPriceCents));
  if (!orderOk || !order || !order.payment || !order.payment.payment_id) {
    check(orderRes, { 'webhook status 200': function () { return false; } });
    check(orderRes, { 'webhook replay status 200': function () { return false; } });
    return;
  }

  const eventId = replayEventId();
  const paymentId = order.payment.payment_id;
  const first = postWebhook(eventId, paymentId);
  check(first, {
    'webhook status 200': function (res) {
      return res.status === 200;
    },
  });

  const second = postWebhook(eventId, paymentId, 'POST /v1/payments/webhooks replay');
  check(second, {
    'webhook replay status 200': function (res) {
      return res.status === 200;
    },
  });
}

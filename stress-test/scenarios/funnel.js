import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { config } from '../lib/config.js';
import { orderChecks, raffleChecks } from '../lib/contract.js';
import { createOrder, getRaffle, postWebhook } from '../lib/http.js';
import { stressEmail, uniqueEventId } from '../lib/ids.js';
import { readJson, ticketsIssued } from '../lib/json.js';
import { pollUntilTickets } from '../lib/poll.js';

const ticketIssueWait = config.scenario === 'funnel'
  ? new Trend('ticket_issue_wait_ms', true)
  : null;

export function funnel(data) {
  const email = stressEmail();
  const listed = getRaffle(data.slug);
  check(listed, raffleChecks(readJson(listed), data.slug));

  const orderRes = createOrder(data.slug, email, config.quantity);
  const order = readJson(orderRes);
  const orderOk = check(orderRes, orderChecks(order, email, config.quantity, data.ticketPriceCents));
  if (!orderOk || !order || !order.payment || !order.payment.payment_id) {
    check(orderRes, {
      'webhook status 200': function () { return false; },
      'tickets length matches quantity': function () { return false; },
      'ticket numbers are strings': function () { return false; },
    });
    return;
  }

  const paymentId = order.payment.payment_id;
  const hook = postWebhook(uniqueEventId(paymentId), paymentId);
  const hookOk = check(hook, {
    'webhook status 200': function (res) {
      return res.status === 200;
    },
  });
  if (!hookOk) {
    check(hook, {
      'tickets length matches quantity': function () { return false; },
      'ticket numbers are strings': function () { return false; },
    });
    return;
  }

  const polled = pollUntilTickets(email, order.order_id, config.quantity);
  if (ticketIssueWait) {
    ticketIssueWait.add(polled.waitedMs);
  }
  if (!polled.ready) {
    console.log(
      'tickets not ready email=' + email +
      ' order_id=' + order.order_id +
      ' count=' + polled.count +
      ' expected=' + config.quantity +
      ' expired=' + polled.expired +
      ' waited_ms=' + polled.waitedMs,
    );
  }
  check(polled, {
    'tickets length matches quantity': function (result) {
      return result.ready === true;
    },
    'ticket numbers are strings': function (result) {
      return result.ready === true && ticketsIssued(result.tickets);
    },
  });
}

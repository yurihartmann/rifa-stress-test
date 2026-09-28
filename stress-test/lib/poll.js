import { sleep } from 'k6';
import { config } from './config.js';
import { getPurchases } from './http.js';
import { purchaseForOrder, readJson, ticketsForOrder } from './json.js';

function result(ready, count, started, expired, tickets) {
  return {
    ready: ready,
    count: count,
    waitedMs: Date.now() - started,
    expired: expired,
    tickets: tickets,
  };
}

export function pollUntilTickets(email, orderId, quantity) {
  const started = Date.now();
  const deadline = started + config.pollTimeoutMs;
  let count = 0;
  let tickets = [];

  while (true) {
    const res = getPurchases(email);
    const body = readJson(res);
    tickets = ticketsForOrder(body, orderId);
    const purchase = purchaseForOrder(body, orderId);
    count = tickets.length;

    if (res.status === 200 && count === quantity) {
      return result(true, count, started, false, tickets);
    }
    if (purchase && purchase.status === 'expired') {
      return result(false, count, started, true, tickets);
    }
    if (Date.now() >= deadline) {
      return result(false, count, started, false, tickets);
    }
    sleep(config.pollIntervalSec);
  }
}

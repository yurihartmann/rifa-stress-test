import { check } from 'k6';
import { config } from '../lib/config.js';
import { orderChecks } from '../lib/contract.js';
import { createOrder } from '../lib/http.js';
import { stressEmail } from '../lib/ids.js';
import { readJson } from '../lib/json.js';

export function checkout(data) {
  const email = stressEmail();
  const res = createOrder(data.slug, email, config.quantity);
  check(res, orderChecks(readJson(res), email, config.quantity, data.ticketPriceCents));
}

import http from 'k6/http';
import { config } from './config.js';

function join(path) {
  return config.baseUrl + path;
}

function expectedCallback(statuses) {
  if (!statuses || statuses.length === 0) {
    return null;
  }
  if (statuses.length === 1) {
    return http.expectedStatuses(statuses[0]);
  }
  return http.expectedStatuses(statuses[0], statuses[1]);
}

function params(name, headers, expectedStatuses) {
  const out = {
    headers: headers,
    timeout: config.httpTimeout,
    tags: { name: name },
  };
  const callback = expectedCallback(expectedStatuses);
  if (callback) {
    out.responseCallback = callback;
  }
  return out;
}

const jsonHeaders = {
  Accept: 'application/json',
  'Content-Type': 'application/json',
};

function adminHeaders() {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    Authorization: 'Bearer ' + config.adminToken,
  };
}

function webhookHeaders() {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    'X-Webhook-Secret': config.webhookSecret,
  };
}

export function getRaffle(slug) {
  return http.get(
    join('/v1/raffles/' + encodeURIComponent(slug)),
    params('GET /v1/raffles/{slug}', { Accept: 'application/json' }),
  );
}

export function createOrder(slug, email, quantity, expectedStatuses) {
  return http.post(
    join('/v1/raffles/' + encodeURIComponent(slug) + '/orders'),
    JSON.stringify({ email: email, quantity: quantity }),
    params('POST /v1/raffles/{slug}/orders', jsonHeaders, expectedStatuses),
  );
}

export function postWebhook(eventId, paymentId, metricName) {
  return http.post(
    join('/v1/payments/webhooks'),
    JSON.stringify({
      event_id: eventId,
      payment_id: paymentId,
      status: 'paid',
    }),
    params(metricName || 'POST /v1/payments/webhooks', webhookHeaders()),
  );
}

export function getPurchases(email) {
  return http.get(
    join('/v1/purchases?email=' + encodeURIComponent(email)),
    params('GET /v1/purchases', { Accept: 'application/json' }),
  );
}

export function createRaffle(payload) {
  return http.post(
    join('/v1/admin/raffles'),
    JSON.stringify(payload),
    params('POST /v1/admin/raffles', adminHeaders()),
  );
}

export function openRaffle(id) {
  return http.patch(
    join('/v1/admin/raffles/' + encodeURIComponent(id)),
    JSON.stringify({ status: 'open' }),
    params('PATCH /v1/admin/raffles/{id}', adminHeaders()),
  );
}

import { config } from './config.js';
import { createRaffle, getRaffle, openRaffle } from './http.js';
import { readJson } from './json.js';

function failure(action, res) {
  const body = res && res.body ? String(res.body).slice(0, 500) : '';
  const error = res && res.error ? res.error : '';
  const status = res ? res.status : 'none';
  return new Error(action + ' failed: status=' + status + ' error=' + error + ' body=' + body);
}

function requireOpenRaffle(body, slug) {
  if (!body || typeof body.id !== 'string' || body.id === '') {
    throw new Error('raffle response missing id for slug ' + slug);
  }
  if (body.status !== 'open') {
    throw new Error('raffle ' + slug + ' is ' + body.status + ', expected open');
  }
  if (typeof body.total_tickets !== 'number' || typeof body.ticket_price_cents !== 'number') {
    throw new Error('raffle response missing total_tickets or ticket_price_cents');
  }
  return {
    slug: body.slug || slug,
    id: body.id,
    totalTickets: body.total_tickets,
    ticketPriceCents: body.ticket_price_cents,
  };
}

export function prepareRaffle() {
  if (config.raffleSlug) {
    const res = getRaffle(config.raffleSlug);
    if (!res || res.status !== 200) {
      throw failure('GET /v1/raffles/' + config.raffleSlug, res);
    }
    return requireOpenRaffle(readJson(res), config.raffleSlug);
  }

  const slug = 'k6-' + Date.now() + '-' + Math.floor(Math.random() * 1000000);
  const created = createRaffle({
    slug: slug,
    title: 'Rifa de carga',
    description: 'Criada pelo setup do k6',
    ticket_price_cents: config.ticketPriceCents,
    total_tickets: config.totalTickets,
  });
  if (!created || created.status !== 201) {
    throw failure('POST /v1/admin/raffles', created);
  }

  const createdBody = readJson(created);
  if (!createdBody || typeof createdBody.id !== 'string' || createdBody.id === '') {
    throw new Error('POST /v1/admin/raffles response missing id');
  }

  const opened = openRaffle(createdBody.id);
  if (!opened || opened.status !== 200) {
    throw failure('PATCH /v1/admin/raffles/' + createdBody.id, opened);
  }

  const listed = getRaffle(slug);
  if (!listed || listed.status !== 200) {
    throw failure('GET /v1/raffles/' + slug, listed);
  }
  return requireOpenRaffle(readJson(listed), slug);
}

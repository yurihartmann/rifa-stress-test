import { check } from 'k6';
import { getRaffle, createOrder } from '../lib/http.js';
import { stressEmail } from '../lib/ids.js';
import { readJson } from '../lib/json.js';

export function overCapacity(data) {
  const listed = getRaffle(data.slug);
  const raffle = readJson(listed);
  const available = raffle && typeof raffle.available_tickets === 'number';
  const listedOk = check(listed, {
    'vitrine status 200': function (res) {
      return res.status === 200;
    },
    'available_tickets': function () {
      return available;
    },
  });
  if (!listedOk) {
    check(listed, {
      'over capacity status 409': function () { return false; },
      'error code insufficient_tickets': function () { return false; },
    });
    return;
  }

  const quantity = raffle.available_tickets + 1;
  const res = createOrder(data.slug, stressEmail(), quantity, [409]);
  const err = readJson(res);
  check(res, {
    'over capacity status 409': function (response) {
      return response.status === 409;
    },
    'error code insufficient_tickets': function () {
      return err && err.code === 'insufficient_tickets';
    },
    'error message': function () {
      return err && typeof err.message === 'string';
    },
  });
}

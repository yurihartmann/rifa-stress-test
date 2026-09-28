export function readJson(res) {
  if (!res || !res.body) {
    return null;
  }
  try {
    return res.json();
  } catch (err) {
    return null;
  }
}

export function purchaseForOrder(body, orderId) {
  if (!body || !Array.isArray(body.purchases)) {
    return null;
  }
  for (let i = 0; i < body.purchases.length; i++) {
    const purchase = body.purchases[i];
    if (purchase && purchase.order_id === orderId) {
      return purchase;
    }
  }
  return null;
}

export function ticketsForOrder(body, orderId) {
  const purchase = purchaseForOrder(body, orderId);
  if (!purchase || !Array.isArray(purchase.tickets)) {
    return [];
  }
  return purchase.tickets;
}

export function ticketsIssued(tickets) {
  if (!Array.isArray(tickets) || tickets.length === 0) {
    return false;
  }
  for (let i = 0; i < tickets.length; i++) {
    const ticket = tickets[i];
    if (!ticket || typeof ticket.number !== 'string' || ticket.number.length === 0) {
      return false;
    }
  }
  return true;
}

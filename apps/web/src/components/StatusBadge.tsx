function badgeTone(status: string): string {
  switch (status) {
    case "open":
    case "paid":
      return "ok";
    case "pending_payment":
    case "draft":
      return "wait";
    default:
      return "muted";
  }
}

export function StatusBadge({ status, label }: { status: string; label: string }) {
  return <span className={`badge badge-${badgeTone(status)}`}>{label}</span>;
}

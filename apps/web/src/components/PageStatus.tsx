type PageStatusProps = {
  kind: "loading" | "error" | "empty";
  title: string;
  detail: string;
};

export function PageStatus({ kind, title, detail }: PageStatusProps) {
  return (
    <div className={`status status-${kind}`} role={kind === "error" ? "alert" : "status"}>
      <h2>{title}</h2>
      <p>{detail}</p>
    </div>
  );
}

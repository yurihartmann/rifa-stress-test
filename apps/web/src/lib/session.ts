import { useCallback, useState } from "react";

export const sessionKeys = {
  adminToken: "rifa.adminToken",
  webhookSecret: "rifa.webhookSecret",
} as const;

export function readSession(key: string): string {
  try {
    return sessionStorage.getItem(key) ?? "";
  } catch {
    return "";
  }
}

export function writeSession(key: string, value: string): void {
  try {
    if (value.length === 0) sessionStorage.removeItem(key);
    else sessionStorage.setItem(key, value);
  } catch {
    // sessionStorage pode estar bloqueado; o valor continua só na memória da página.
  }
}

export function useSessionValue(key: string): [string, (value: string) => void] {
  const [value, setValue] = useState(() => readSession(key));
  const update = useCallback(
    (next: string) => {
      writeSession(key, next);
      setValue(next);
    },
    [key],
  );
  return [value, update];
}

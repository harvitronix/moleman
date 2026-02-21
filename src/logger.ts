export type LogLevel = "debug" | "info" | "warn" | "error";

const order: Record<LogLevel, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40,
};

let currentLevel: LogLevel = "info";

export function setLogLevel(level: LogLevel): void {
  currentLevel = level;
}

function shouldLog(level: LogLevel): boolean {
  return order[level] >= order[currentLevel];
}

function timestamp(): string {
  const now = new Date();
  const hh = String(now.getHours()).padStart(2, "0");
  const mm = String(now.getMinutes()).padStart(2, "0");
  const ss = String(now.getSeconds()).padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}

function formatFields(fields?: Record<string, unknown>): string {
  if (!fields) {
    return "";
  }
  const parts = Object.entries(fields).map(([key, value]) => `${key}=${String(value)}`);
  return parts.length > 0 ? ` ${parts.join(" ")}` : "";
}

function write(level: LogLevel, message: string, fields?: Record<string, unknown>): void {
  if (!shouldLog(level)) {
    return;
  }

  const line = `${timestamp()} moleman ${level.toUpperCase()} ${message}${formatFields(fields)}`;
  if (level === "error" || level === "warn") {
    process.stderr.write(`${line}\n`);
    return;
  }
  process.stdout.write(`${line}\n`);
}

export function debug(message: string, fields?: Record<string, unknown>): void {
  write("debug", message, fields);
}

export function info(message: string, fields?: Record<string, unknown>): void {
  write("info", message, fields);
}

export function warn(message: string, fields?: Record<string, unknown>): void {
  write("warn", message, fields);
}

export function error(message: string, fields?: Record<string, unknown>): void {
  write("error", message, fields);
}

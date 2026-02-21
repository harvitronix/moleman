import { evaluateExpression } from "./expr.js";

export function renderTemplate(input: string, data: Record<string, unknown>): string {
  if (input.length === 0) {
    return "";
  }

  try {
    return input.replace(/{{([\s\S]*?)}}/g, (_match, rawExpr: string) => {
      const value = evalTemplateExpression(rawExpr.trim(), data);
      return stringifyTemplateValue(value);
    });
  } catch (err) {
    if (err instanceof Error) {
      throw new Error(`execute template: ${err.message}`);
    }
    throw err;
  }
}

export function shellEscape(input: string): string {
  if (input.length === 0) {
    return "''";
  }
  return `'${input.replaceAll("'", "'\"'\"'")}'`;
}

function evalTemplateExpression(expr: string, data: Record<string, unknown>): unknown {
  if (expr.length === 0) {
    return "";
  }

  if (expr.startsWith("shellEscape ")) {
    const arg = expr.slice("shellEscape ".length).trim();
    const rendered = evalTemplateExpression(arg, data);
    return shellEscape(rendered === undefined || rendered === null ? "" : String(rendered));
  }

  if (expr.startsWith("index ")) {
    const args = splitArgs(expr.slice("index ".length).trim());
    if (args.length !== 2) {
      throw new Error("index requires exactly two arguments");
    }
    const base = evalTemplateExpression(args[0], data);
    const key = evalTemplateExpression(args[1], data);
    return indexValue(base, key);
  }

  if (isQuotedLiteral(expr)) {
    return parseQuoted(expr);
  }

  if (/^-?\d+(?:\.\d+)?$/.test(expr)) {
    return Number(expr);
  }

  if (expr === "true") {
    return true;
  }

  if (expr === "false") {
    return false;
  }

  if (expr === ".") {
    return data;
  }

  let normalized = expr;
  if (normalized.startsWith(".")) {
    normalized = normalized.slice(1);
  }

  if (normalized.length === 0) {
    return data;
  }

  return evaluateExpression(normalized, data, { missing: "undefined" });
}

function indexValue(base: unknown, key: unknown): unknown {
  if (base === null || base === undefined) {
    return undefined;
  }

  if (Array.isArray(base)) {
    const numeric = typeof key === "number" ? key : Number(key);
    if (!Number.isInteger(numeric) || numeric < 0 || numeric >= base.length) {
      return undefined;
    }
    return base[numeric];
  }

  if (typeof base === "object") {
    return (base as Record<string, unknown>)[String(key)];
  }

  return undefined;
}

function splitArgs(input: string): string[] {
  const args: string[] = [];
  let current = "";
  let quote: string | null = null;

  for (let i = 0; i < input.length; i += 1) {
    const char = input[i];

    if (quote) {
      current += char;
      if (char === "\\" && i + 1 < input.length) {
        i += 1;
        current += input[i];
        continue;
      }
      if (char === quote) {
        quote = null;
      }
      continue;
    }

    if (char === '"' || char === "'") {
      quote = char;
      current += char;
      continue;
    }

    if (/\s/.test(char)) {
      if (current.length > 0) {
        args.push(current);
        current = "";
      }
      continue;
    }

    current += char;
  }

  if (quote) {
    throw new Error("unterminated string literal");
  }

  if (current.length > 0) {
    args.push(current);
  }

  return args;
}

function isQuotedLiteral(value: string): boolean {
  return (value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"));
}

function parseQuoted(value: string): string {
  if (value.startsWith('"')) {
    return JSON.parse(value) as string;
  }

  const body = value.slice(1, -1);
  return body.replace(/\\'/g, "'").replace(/\\\\/g, "\\");
}

function stringifyTemplateValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
}

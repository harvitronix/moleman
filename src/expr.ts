export interface EvalOptions {
  missing: "throw" | "undefined";
}

const defaultEvalOptions: EvalOptions = {
  missing: "throw",
};

export function evalCondition(expr: string, data: Record<string, unknown>): boolean {
  const normalized = normalizeExpression(expr);

  try {
    const value = evaluateExpression(normalized, data, { missing: "throw" });
    if (typeof value !== "boolean") {
      throw new Error("condition did not evaluate to bool");
    }
    return value;
  } catch (err) {
    if (isMissingValueError(err)) {
      return false;
    }
    throw err;
  }
}

export function evaluateExpression(
  expr: string,
  data: Record<string, unknown>,
  options: EvalOptions = defaultEvalOptions,
): unknown {
  const normalized = normalizeExpression(expr);

  try {
    return runExpression(normalized, data);
  } catch (err) {
    if (options.missing === "undefined" && isMissingValueError(err)) {
      return undefined;
    }
    throw err;
  }
}

function normalizeExpression(expr: string): string {
  let normalized = expr.trim();
  if (normalized.startsWith("{{") && normalized.endsWith("}}")) {
    normalized = normalized.slice(2, -2).trim();
  }
  if (!normalized) {
    throw new Error("empty condition");
  }
  return normalized;
}

function runExpression(expr: string, data: Record<string, unknown>): unknown {
  const evaluator = new Function(
    "data",
    `with (data) { return (${expr}); }`,
  ) as (ctx: Record<string, unknown>) => unknown;

  return evaluator(data);
}

function isMissingValueError(err: unknown): boolean {
  if (!(err instanceof Error)) {
    return false;
  }

  return (
    err instanceof ReferenceError ||
    /is not defined/.test(err.message) ||
    /Cannot read properties of undefined/.test(err.message) ||
    /Cannot read properties of null/.test(err.message)
  );
}

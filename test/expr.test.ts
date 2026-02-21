import test from "node:test";
import assert from "node:assert/strict";
import { evalCondition } from "../src/expr.js";

test("evalCondition handles basic expressions", () => {
  const data = {
    outputs: {
      review_json: {
        structured_output: {
          must_fix_count: 0,
        },
      },
      previous: "ok",
    },
    last: "ok",
  };

  const cases: Array<{ expr: string; want: boolean }> = [
    { expr: "outputs.review_json.structured_output.must_fix_count == 0", want: true },
    { expr: "outputs.review_json.structured_output.must_fix_count != 0", want: false },
    { expr: 'last == "ok"', want: true },
    { expr: "true && false", want: false },
    { expr: "true || false", want: true },
    { expr: "{{ outputs.review_json.structured_output.must_fix_count == 1 }}", want: false },
  ];

  for (const tc of cases) {
    assert.equal(evalCondition(tc.expr, data), tc.want, tc.expr);
  }
});

test("evalCondition returns false for missing and non-matching expressions", () => {
  const data = {
    outputs: {
      bad: "zero",
      obj: {
        value: "nope",
      },
    },
  };

  assert.equal(evalCondition("outputs.missing == 1", data), false);
  assert.equal(evalCondition("outputs.bad == 1", data), false);
  assert.equal(evalCondition('outputs.obj == "zero"', data), false);

  assert.throws(() => evalCondition("", data));
});

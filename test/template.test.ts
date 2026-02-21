import test from "node:test";
import assert from "node:assert/strict";
import { renderTemplate, shellEscape } from "../src/template.js";

test("renderTemplate supports value interpolation", () => {
  const data = {
    input: {
      prompt: "hello",
    },
    outputs: {
      review_json: {
        structured_output: {
          must_fix_count: 0,
        },
      },
    },
  };

  const out = renderTemplate("{{ .input.prompt }} {{ .outputs.review_json.structured_output.must_fix_count }}", data);
  assert.equal(out, "hello 0");
});

test("renderTemplate supports index helper", () => {
  const data = {
    outputs: {
      fix: "done",
    },
  };
  const out = renderTemplate("{{ index .outputs \"fix\" }}", data);
  assert.equal(out, "done");
});

test("shellEscape matches expected format", () => {
  assert.equal(shellEscape("we're good"), "'we'\"'\"'re good'");
});

test("renderTemplate handles empty input", () => {
  const out = renderTemplate("", { ignored: true });
  assert.equal(out, "");
});

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Field } from "./ui";

/** The field rendered: its input's attributes, what its label names, and the
 * texts its aria-describedby points at, in order. */
const render = (hint?: string, refusal?: string) => {
  const html = renderToStaticMarkup(
    <Field label="Name" hint={hint} refusal={refusal}>
      {(input) => <input {...input} />}
    </Field>,
  );
  const attr = (name: string) => html.match(new RegExp(`<input[^>]* ${name}="([^"]*)"`))?.[1];
  const textOf = (id: string) => html.match(new RegExp(`id="${id}"[^>]*>([^<]*)<`))?.[1];
  const labelled = html.match(/<label for="([^"]*)"[^>]*>([^<]*)<\/label>/);
  return {
    id: attr("id"),
    invalid: attr("aria-invalid"),
    labelFor: labelled?.[1],
    label: labelled?.[2],
    described: (attr("aria-describedby") ?? "").split(" ").filter(Boolean).map(textOf),
  };
};

// A screen reader names the field by its label, then reads its hint and its
// refusal, which never sit inside the label (spec/repo-display-names-ux.md).
describe("a form field", () => {
  it("is named by its label alone, its hint read as its description", () => {
    const f = render("Its id: ledger-api.");
    expect(f.labelFor).toBe(f.id);
    expect(f.label).toBe("Name");
    expect(f.described).toEqual(["Its id: ledger-api."]);
    expect(f.invalid).toBeUndefined();
  });

  it("reads a refusal after the hint, and marks the input invalid", () => {
    const f = render("Its id: hq.", "“HQ” is the project’s HQ.");
    expect(f.described).toEqual(["Its id: hq.", "“HQ” is the project’s HQ."]);
    expect(f.invalid).toBe("true");
  });

  it("has no description without a hint", () => {
    expect(render().described).toEqual([]);
  });
});

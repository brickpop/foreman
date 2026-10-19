// Foreman's extension for Pi (internal/harness/pi), written by the adapter
// with the program's path filled in.
//
// Hooks: Pi has no shell hooks, so this relays the events Foreman follows to
// `foreman-agent hook <event>`, their JSON on stdin, as Claude Code's hooks
// do. Each handler waits for its relay, so the events reach Foreman in order.
//
// ask_user: Pi has no tool to ask the user, as Claude Code's AskUserQuestion
// does; this one asks in a dialog in Pi's terminal, and its opening and
// closing (ui_prompt_start, ui_prompt_end) are what Foreman reads as waiting.
import { spawn } from "node:child_process";
import { Type } from "typebox";

const program: string = FOREMAN_AGENT;

function hook(event: string, payload: object): Promise<void> {
  return new Promise((resolve) => {
    const child = spawn(program, ["hook", event], { stdio: ["pipe", "ignore", "ignore"], timeout: 5000 });
    child.on("error", () => resolve());
    child.on("close", () => resolve());
    child.stdin.on("error", () => {});
    child.stdin.end(JSON.stringify({ ...payload, cwd: process.cwd() }));
  });
}

const other = "Something else (type it)";

// biome-ignore lint/suspicious/noExplicitAny: Pi's types aren't installed here
export default function (pi: any) {
  pi.on("session_start", () => hook("session_start", {}));
  // A run begins: a message taken (a slash command starts none).
  pi.on("before_agent_start", (e: { prompt?: string }) => hook("before_agent_start", { prompt: e.prompt ?? "" }));
  pi.on("tool_call", (e: { toolName: string; input: unknown }) =>
    hook("tool_call", { toolName: e.toolName, input: e.input }),
  );
  pi.on("ui_prompt_start", (e: { title?: string }) => hook("ui_prompt_start", { title: e.title ?? "" }));
  pi.on("ui_prompt_end", () => hook("ui_prompt_end", {}));
  // Final: Pi won't continue on its own (agent_end can be followed by retries).
  pi.on("agent_settled", (e: { aborted: boolean }) => hook("agent_settled", { aborted: e.aborted }));

  pi.registerTool({
    name: "ask_user",
    label: "Ask the user",
    description:
      "Ask the user a question and wait for the answer. Give a few options when the answer is a choice; the user can always answer something else. Use it only when you can't go on without the user's decision.",
    parameters: Type.Object({
      question: Type.String({ description: "The question, complete on its own" }),
      options: Type.Optional(Type.Array(Type.String(), { description: "The answers to choose from, if it is a choice" })),
    }),
    exposure: "model-only",
    executionMode: "sequential",
    async execute(
      _id: string,
      params: { question: string; options?: string[] },
      _signal: unknown,
      _onUpdate: unknown,
      // biome-ignore lint/suspicious/noExplicitAny: Pi's types aren't installed here
      ctx: any,
    ) {
      let answer: string | undefined;
      if (params.options?.length) {
        answer = await ctx.ui.select(params.question, [...params.options, other]);
        if (answer === other) answer = await ctx.ui.input(params.question);
      } else {
        answer = await ctx.ui.input(params.question);
      }
      const text = answer?.trim() ? `The user answered: ${answer.trim()}` : "The user dismissed the question without answering.";
      return { content: [{ type: "text", text }], details: { question: params.question, answer: answer ?? null } };
    },
  });
}

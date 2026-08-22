# Model providers

Two implementations reach the whole field, because the protocol matters more
than the vendor.

| `LLM_PROVIDER` | Speaks | Reaches |
|---|---|---|
| `openai` | chat completions | OpenAI, Azure OpenAI, LM Studio, Ollama, vLLM, llama.cpp server, LiteLLM |
| `anthropic` | Messages API | Anthropic, and Bedrock/Vertex or LiteLLM gateways presenting the same shape |

`LLM_BASE_URL` is required for `openai` and optional for `anthropic`. That one
value is what makes a self-hosted model a first-class path rather than a
workaround.

**There is no default provider.** A component that installs cleanly and then
quietly spends money against a vendor the operator did not choose is a bad
default, so `LLM_PROVIDER` and `LLM_MODEL` must both be set.

## Structured output

Both implementations force the model to answer through the `Verdict` schema —
`response_format: json_schema` for chat completions, a single forced tool call
for Messages. Where the backend honours it, a malformed answer is impossible
rather than merely unlikely.

## Reasoning models put the answer somewhere else

Verified against LM Studio serving `qwen3.6-35b-a3b`: the schema-constrained
JSON arrived in `message.reasoning_content` with `message.content` **empty**.
A client reading only `content` sees nothing and reports a broken model.

The `openai` implementation tries `content`, then `reasoning_content`, then
`reasoning`, and parses the first that yields a valid verdict. If you add a
provider, do the same — this is not exotic, it is how most llama.cpp-derived
servers behave with a reasoning model.

`LLM_REASONING_EFFORT` passes through where supported. Leave it unset for
models that do not.

## Choosing a model

The task is small and the output is checked, so this does not need a large
model. Measured on the nine-case eval with a **9B**: 8/9 classification, 8/9
full pass, **0 unsafe**.

Score in this order:

1. **UNSAFE must be zero.** Anything above zero disqualifies a model at any
   accuracy — it means something wrong reached the repository.
2. **Classification** — how often the judgement is right.
3. **Full pass** — whether exactly the right edits landed.

A model with mediocre classification and zero unsafe is usable: it escalates
more than it needs to, which costs a human two minutes.

## Adding a provider

Implement one method:

```go
Classify(ctx context.Context, systemPrompt, userPrompt string) (*Verdict, error)
```

Constrain the model to `VerdictSchema()` if the backend can. Call `Verdict.Valid()`
before returning. Do not retry indefinitely — the caller is asynchronous but
not patient, and a wedged provider should surface as a comment rather than a
hang.

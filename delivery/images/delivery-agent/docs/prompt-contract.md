# The prompt contract

The agent's job is narrow and its output is checked, which is what makes a
small local model viable. This is the reasoning behind the prompt in
`prompt.go`, and the measurements that produced it.

## The problem is the edit format, not the judgement

Classification turns out to be easy. Every model tried gets "is this a chart
default flip or a one-way migration" right almost every time.

Producing a *usable edit* is where small models fail. Asked for "the fix",
a model will return:

- a file path in the `key` field,
- several lines of YAML in `from` and `to`,
- a paraphrase of the current value rather than the value.

All three are rejected by the applier, so nothing gets fixed and the run is
wasted. Every lever below exists to prevent that.

## Lever 1 — hand it an inventory, not a file

The single biggest change. Instead of pasting file contents, the agent
extracts every scalar and presents key/value pairs in exactly the form an edit
must use:

```
FILE addons/environments/production/addons/addons.yaml -- editable scalars (key = value):
  metallb.defaultVersion = 0.16.0
  metallb.valuesObject.speaker.frr.enabled = true
  metallb.valuesObject.frrk8s.enabled = true
```

This converts *generation* into *selection*. A key that does not exist becomes
inexpressible, and `from` is copied from text the model was just shown rather
than reconstructed — so the applier's equality check passes instead of
rejecting a paraphrase.

Measured on a 9B model across the case set: 6/9 full pass without the
inventory, 8/9 with it. The failure it removes is the dangerous one — a
*partial* fix, where one of two required edits lands and the result renders
green while still being wrong.

## Lever 2 — spell out the contract with worked examples

The schema descriptions alone are not enough; models skip them. The prompt
carries correct and incorrect examples side by side, and names the four ways an
edit gets rejected. That turned the malformed-shape failure from routine into
absent.

## Lever 3 — never invent a version, enforced in code

A model told *"requires Gateway API v1.5"* will confidently write `v1.5.0` when
the answer was `v1.5.1`. This is the worst failure available to us: it renders
perfectly and breaks at runtime.

Telling the model not to do this **does not work**. Measured: with an explicit
rule in the prompt forbidding it, a 9B model still invented the patch version.

So the guarantee lives in `edits.Policy.Evidence`. Any version-shaped value an
edit writes must appear verbatim in the material the model was shown. It does
not, the edit is refused, and the run escalates instead.

Only version-shaped values are corroborated. Booleans and ports are exempt on
purpose — `"false"` rarely appears in a failure report, and corroborating it
would reject the most common mechanical fix there is.

## Lever 4 — an empty result is an escalation

If a `mechanical` verdict produces zero applied edits — every one rejected —
the agent escalates rather than reporting success. This is what converts
miscalibration into a safe outcome automatically: the model can be wrong about
the classification, and the result is still a human being asked.

## What the model is never trusted with

Neither the prompt nor the model decides any of this:

| Guarantee | Enforced by |
|---|---|
| Cannot edit the gate, CI, or the merge policy | path deny-list, before any write |
| Cannot edit outside the configured area | path allow-list |
| Cannot overwrite a value it misread | `from` must match the file |
| Cannot invent a version | corroboration against the evidence |
| Cannot add keys, only change them | the key must already resolve to a scalar |
| Cannot escape the repository | path traversal check |
| Cannot try forever | attempt cap, tracked by label |

That table is the reason a 9B model is an acceptable choice here. It is not
that the model is reliable — it is that being wrong is cheap and being
dangerous is impossible.

## Re-running the measurements

The eval cases are real incidents, not invented ones. Run them against any
OpenAI-compatible endpoint:

```bash
DELIVERY_AGENT_LIVE=http://localhost:1234/v1 \
DELIVERY_AGENT_MODELS=your-model \
DELIVERY_AGENT_PROMPT="$(scripts/extract-prompt.sh)" \
go test ./evals -run Eval -v -timeout 60m
```

Add `DELIVERY_AGENT_NO_INVENTORY=1` to reproduce the lever-1 ablation.

Score three things, in order of importance:

1. **UNSAFE** — did anything wrong land on disk? This must be zero.
2. **classification** — is the judgement right?
3. **full pass** — did exactly the right edits land?

A model with UNSAFE 0 is usable even if its classification is mediocre; a model
with UNSAFE above 0 is not usable at any accuracy.

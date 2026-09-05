# Frontend performance tooling

Measures how long a single keyboard interaction (`j`/`k` article navigation)
actually takes, end to end, against real data in the dev stack.

## Why these exist

Eyeballing "it feels laggy" does not survive a refactor. These let a change be
stated as a number: keydown to paint, and whether the frame blew the 50ms
long-task budget.

## Quick start

```bash
# 1. Bring up the dev stack (rootless Docker on this host)
export DOCKER_HOST=unix:///run/user/1000/docker.sock
goliath-cli up --env dev --build

# 2. Put every article back to unread, so runs are comparable
tools/perf/reset-read-state.sh

# 3. Optional: build and serve a production bundle on :4173.
#    Dev-mode React/MUI/emotion carry real overhead; measure prod for headline
#    numbers and dev for fast iteration.
tools/perf/serve-prod-preview.sh
```

Then, in the browser console on <http://localhost:3000> (dev) or
<http://localhost:4173> (prod preview), after the article list has rendered:

```js
const b = await import('/tools/perf/keypress-bench.js');
const r = await b.runBench({ key: 'j', presses: 60, warmup: 5, gapMs: 200 });
b.summarize(r);
```

Reload the page after each `reset-read-state.sh` so the in-memory content tree
is rebuilt from the restored database state.

## What it reports

Per press, from `runBench`:

| Field | Meaning |
| --- | --- |
| `sync` | Work inside the keydown listener itself. |
| `toCommit` | Keydown until React has rendered and committed, pre-paint. |
| `toPaint` | Keydown until the browser has painted. This is the felt latency. |
| `jank.loaf*` | Long Animation Frames — the browser's own jank signal, with phase and script attribution. Only frames >= 50ms are reported, so any non-zero count means visible stalls. |
| `jank.longtask*` | Long tasks (>50ms). |
| `traceAnalysis` | Sampled self/total time per function, plus folded stacks for speedscope or flamegraph.pl. |

`microbench()`, `benchGetArticleView()` and `benchDerivedViews()` time specific
model-layer functions exactly, reaching the live `ContentTreeCls` through the
React fiber tree.

## Two traps worth knowing

**Sampled attribution understates React components.** React renders each child
from a work loop, not nested inside its parent's render frame, so a parent never
appears on its children's stacks. A component that owns 67% of the cost can look
like 4%. To attribute cost to a component, *ablate it* — make it `return null`,
re-measure, and diff — rather than trusting the sampled trace.

**Chrome pins `Profiler` to a ~10ms sample interval** no matter what
`sampleInterval` you request, and cross-origin isolation does not change it. The
trace is for discovering unknown hotspots; use LoAF phase data and `microbench`
for numbers you intend to quote.

## Requirements

`frontend/vite.config.js` sets, for the dev server and `vite preview` only:

- `Document-Policy: js-profiling` — required to construct `Profiler`.
- `Cross-Origin-Opener-Policy` / `Cross-Origin-Embedder-Policy: credentialless` —
  cross-origin isolation, which takes `performance.now()` from 100us to 5us.

`compose.yaml` mounts `./tools` into the frontend container and publishes 4173.
None of this reaches a production build; the harness is never imported by the app.

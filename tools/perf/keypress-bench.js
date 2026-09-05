/**
 * Dev-only keypress performance harness for the Goliath frontend.
 *
 * Measures the cost of a single keyboard interaction (j/k article navigation)
 * end to end: synchronous handler, React render + commit, and paint. Captures
 * Long Animation Frames (the browser's own jank signal, with script
 * attribution) and a JS Self-Profiling sampled trace for flame graphs.
 *
 * Never imported by the app. Loaded on demand from the browser console or an
 * automation driver:
 *
 *   const b = await import('/@fs/<repo>/tools/perf/keypress-bench.js');
 *   const r = await b.runBench({ key: 'j', presses: 60 });
 *
 * Requirements (both configured in frontend/vite.config.js, dev server only):
 *   - `Document-Policy: js-profiling` response header, for `new Profiler()`.
 *   - `server.fs.allow` covering the repo root, for the /@fs/ import above.
 */

const SELECTED = '.GoliathArticleCardSelected';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/**
 * Resolves twice around the next paint: `rafAt` is inside the rAF callback
 * (render + commit are done, paint has not happened yet), `paintedAt` is in a
 * task queued from that callback, which the browser runs after it paints.
 */
function afterPaint() {
  return new Promise((resolve) => {
    requestAnimationFrame(() => {
      const rafAt = performance.now();
      setTimeout(() => resolve({ rafAt, paintedAt: performance.now() }), 0);
    });
  });
}

function selectedTitle() {
  const el = document.querySelector(`${SELECTED} .GoliathArticleCardTitle`);
  return el ? el.textContent.trim() : null;
}

function quantile(sorted, q) {
  if (sorted.length === 0) return NaN;
  const pos = (sorted.length - 1) * q;
  const lo = Math.floor(pos);
  const hi = Math.ceil(pos);
  return lo === hi ? sorted[lo] : sorted[lo] + (sorted[hi] - sorted[lo]) * (pos - lo);
}

function stats(values) {
  if (values.length === 0) return null;
  const s = [...values].sort((a, b) => a - b);
  const mean = s.reduce((a, b) => a + b, 0) / s.length;
  const variance = s.reduce((a, b) => a + (b - mean) ** 2, 0) / s.length;
  const round = (x) => Math.round(x * 100) / 100;
  return {
    n: s.length,
    mean: round(mean),
    sd: round(Math.sqrt(variance)),
    min: round(s[0]),
    p50: round(quantile(s, 0.5)),
    p90: round(quantile(s, 0.9)),
    p99: round(quantile(s, 0.99)),
    max: round(s[s.length - 1]),
  };
}

function serializeLoaf(e) {
  return {
    startTime: e.startTime,
    duration: e.duration,
    blockingDuration: e.blockingDuration,
    renderStart: e.renderStart,
    styleAndLayoutStart: e.styleAndLayoutStart,
    scripts: (e.scripts || []).map((s) => ({
      name: s.name,
      invoker: s.invoker,
      invokerType: s.invokerType,
      duration: s.duration,
      forcedStyleAndLayoutDuration: s.forcedStyleAndLayoutDuration,
      sourceURL: s.sourceURL,
      sourceFunctionName: s.sourceFunctionName,
    })),
  };
}

/**
 * Flattens a JS Self-Profiling trace into per-function self/total times and
 * folded stacks (the `a;b;c <count>` format speedscope and flamegraph.pl read).
 */
export function analyzeTrace(trace) {
  const { frames, stacks, samples, resources } = trace;
  const totalSamples = samples.length;
  if (totalSamples === 0) return { totalSamples: 0, self: [], folded: '' };

  // Wall-clock weight of one sample, derived from the trace's own timestamps.
  const span = samples[totalSamples - 1].timestamp - samples[0].timestamp;
  const msPerSample = totalSamples > 1 ? span / (totalSamples - 1) : 0;

  const label = (frameId) => {
    const f = frames[frameId];
    if (!f) return '(unknown)';
    const name = f.name || '(anonymous)';
    const res = f.resourceId != null && resources ? resources[f.resourceId] : null;
    const file = res ? res.replace(/^https?:\/\/[^/]+/, '').split('?')[0] : '';
    const line = f.line != null ? `:${f.line}` : '';
    return file ? `${name} (${file}${line})` : name;
  };

  // Walk a stack node up to the root, returning leaf-last frame labels.
  const stackPath = (stackId) => {
    const out = [];
    let id = stackId;
    while (id != null) {
      const node = stacks[id];
      if (!node) break;
      out.push(label(node.frameId));
      id = node.parentId;
    }
    return out.reverse();
  };

  const pathCache = new Map();
  const selfMs = new Map();
  const totalMs = new Map();
  const foldedCounts = new Map();

  for (const sample of samples) {
    if (sample.stackId == null) {
      // No JS on the stack — the sample landed in browser-internal work.
      foldedCounts.set('(idle/native)', (foldedCounts.get('(idle/native)') || 0) + 1);
      continue;
    }
    let path = pathCache.get(sample.stackId);
    if (!path) {
      path = stackPath(sample.stackId);
      pathCache.set(sample.stackId, path);
    }
    const leaf = path[path.length - 1];
    selfMs.set(leaf, (selfMs.get(leaf) || 0) + msPerSample);
    for (const frame of new Set(path)) {
      totalMs.set(frame, (totalMs.get(frame) || 0) + msPerSample);
    }
    const key = path.join(';');
    foldedCounts.set(key, (foldedCounts.get(key) || 0) + 1);
  }

  const self = [...selfMs.entries()]
    .map(([name, ms]) => ({
      name,
      selfMs: Math.round(ms * 100) / 100,
      totalMs: Math.round((totalMs.get(name) || 0) * 100) / 100,
      selfPct: Math.round((ms / (msPerSample * totalSamples)) * 1000) / 10,
    }))
    .sort((a, b) => b.selfMs - a.selfMs);

  const folded = [...foldedCounts.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([k, v]) => `${k} ${v}`)
    .join('\n');

  return { totalSamples, msPerSample, self, folded };
}

/**
 * Drives `presses` synthetic keydowns through the real app and returns timing
 * statistics, LoAF/long-task records, and (optionally) a sampled trace.
 *
 * @param {object}  opts
 * @param {string}  opts.key       Key to dispatch ('j' or 'k').
 * @param {number}  opts.presses   Measured presses (warmup excluded).
 * @param {number}  opts.warmup    Discarded leading presses.
 * @param {number}  opts.gapMs     Idle gap between presses. Keep above the
 *                                 100ms smooth-scroll animation so presses do
 *                                 not overlap each other's work.
 * @param {boolean} opts.profile   Capture a JS Self-Profiling trace. Chrome clamps
 *                                 sampling to a 10ms floor, so the trace is for
 *                                 discovery only — prefer LoAF script attribution
 *                                 and microbench() for exact numbers.
 */
export async function runBench({
  key = 'j',
  presses = 60,
  warmup = 5,
  gapMs = 300,
  profile = true,
  sampleInterval = 10,
} = {}) {
  const loafs = [];
  const longtasks = [];

  const loafObs = new PerformanceObserver((list) => {
    for (const e of list.getEntries()) loafs.push(serializeLoaf(e));
  });
  let loafSupported = true;
  try {
    loafObs.observe({ type: 'long-animation-frame', buffered: false });
  } catch {
    loafSupported = false;
  }

  const ltObs = new PerformanceObserver((list) => {
    for (const e of list.getEntries()) {
      longtasks.push({ startTime: e.startTime, duration: e.duration, name: e.name });
    }
  });
  try {
    ltObs.observe({ type: 'longtask', buffered: false });
  } catch {
    /* not supported */
  }

  let profiler = null;
  let profileError = null;
  if (profile) {
    try {
      profiler = new Profiler({ sampleInterval, maxBufferSize: 1000000 });
    } catch (e) {
      profileError = `${e.name}: ${e.message}`;
    }
  }

  const perPress = [];
  let missedCommits = 0;
  const runStart = performance.now();

  for (let i = 0; i < warmup + presses; i++) {
    const before = selectedTitle();

    const t0 = performance.now();
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
    );
    const tSync = performance.now();

    const { rafAt, paintedAt } = await afterPaint();
    const after = selectedTitle();

    if (i >= warmup) {
      if (after === before) missedCommits++;
      perPress.push({
        index: i - warmup,
        startTime: t0,
        // Synchronous work in the keydown listener itself.
        syncMs: tSync - t0,
        // Keydown until React has rendered and committed (pre-paint).
        toCommitMs: rafAt - t0,
        // Keydown until the browser has painted the new frame.
        toPaintMs: paintedAt - t0,
        advanced: after !== before,
      });
    }
    await sleep(gapMs);
  }

  const runEnd = performance.now();
  loafObs.disconnect();
  ltObs.disconnect();

  let trace = null;
  let traceAnalysis = null;
  if (profiler) {
    trace = await profiler.stop();
    traceAnalysis = analyzeTrace(trace);
  }

  // Only count jank that happened inside the measured window.
  const inWindow = (e) => e.startTime >= runStart && e.startTime <= runEnd;
  const windowLoafs = loafs.filter(inWindow);
  const windowLongtasks = longtasks.filter(inWindow);

  return {
    config: { key, presses, warmup, gapMs, profile, sampleInterval },
    env: {
      articlesInView: document.querySelectorAll('.GoliathArticleCard').length,
      unreadFromTitle: (document.title.match(/\((\d+)\)/) || [])[1] || null,
      hardwareConcurrency: navigator.hardwareConcurrency,
      crossOriginIsolated: window.crossOriginIsolated,
      timerResolutionMs: 0.1,
    },
    wallMs: Math.round(runEnd - runStart),
    missedCommits,
    timing: {
      sync: stats(perPress.map((p) => p.syncMs)),
      toCommit: stats(perPress.map((p) => p.toCommitMs)),
      toPaint: stats(perPress.map((p) => p.toPaintMs)),
    },
    jank: {
      loafSupported,
      // LoAF only reports frames >= 50ms, so these are the visible stalls.
      loafCount: windowLoafs.length,
      loafPerPress: Math.round((windowLoafs.length / presses) * 100) / 100,
      loafDuration: stats(windowLoafs.map((e) => e.duration)),
      loafBlocking: stats(windowLoafs.map((e) => e.blockingDuration)),
      longtaskCount: windowLongtasks.length,
      longtaskDuration: stats(windowLongtasks.map((e) => e.duration)),
    },
    profileError,
    perPress,
    loafs: windowLoafs,
    longtasks: windowLongtasks,
    traceAnalysis,
    trace,
  };
}

/** Compact console summary; returns the same result for further inspection. */
export function summarize(r) {
  const line = (label, s) =>
    s ? `${label.padEnd(10)} n=${s.n} mean=${s.mean}ms p50=${s.p50} p90=${s.p90} p99=${s.p99} max=${s.max}` : `${label}: none`;
  console.log(`--- keypress bench: '${r.config.key}' x${r.config.presses} ---`);
  console.log(line('sync', r.timing.sync));
  console.log(line('toCommit', r.timing.toCommit));
  console.log(line('toPaint', r.timing.toPaint));
  console.log(`LoAF: ${r.jank.loafCount} (${r.jank.loafPerPress}/press)`, r.jank.loafDuration);
  if (r.traceAnalysis) {
    console.log('Top self time:');
    console.table(r.traceAnalysis.self.slice(0, 20));
  }
  return r;
}

// ---------------------------------------------------------------------------
// Direct access to app internals, for exact microbenchmarks of hot functions.
// ---------------------------------------------------------------------------

/** Walks the React fiber tree to find the App class instance. */
export function findAppInstance() {
  const root = document.getElementById('root');
  if (!root) throw new Error('#root not found');
  const key = Object.keys(root).find((k) => k.startsWith('__reactContainer$'));
  if (!key) throw new Error('React root fiber key not found');

  const seen = new Set();
  const stack = [root[key]];
  while (stack.length) {
    const f = stack.pop();
    if (!f || seen.has(f)) continue;
    seen.add(f);
    if (f.stateNode && f.stateNode.state && f.stateNode.state.contentTreeCls) {
      return f.stateNode;
    }
    if (f.child) stack.push(f.child);
    if (f.sibling) stack.push(f.sibling);
  }
  throw new Error('App instance not found in fiber tree');
}

export function getContentTree() {
  return findAppInstance().state.contentTreeCls;
}

/**
 * Times `fn` over `iters` iterations after `warmup` discarded ones. With
 * cross-origin isolation enabled the timer resolves to 5us, so per-iteration
 * numbers are meaningful down to roughly 10us.
 */
export function microbench(name, fn, { iters = 50, warmup = 5 } = {}) {
  for (let i = 0; i < warmup; i++) fn(i);
  const times = [];
  for (let i = 0; i < iters; i++) {
    const t0 = performance.now();
    fn(i);
    times.push(performance.now() - t0);
  }
  return { name, ...stats(times) };
}

/**
 * Measures ContentTreeCls.GetArticleView for the current selection, forcing a
 * full recompute each iteration (the cache is invalidated on every mark, so a
 * cold recompute is what a j-press actually pays for).
 */
export function benchGetArticleView({ iters = 50 } = {}) {
  const app = findAppInstance();
  const ct = app.state.contentTreeCls;
  const { selectionKey, selectionType } = app.state;
  const len = ct.GetArticleView(selectionKey, selectionType).length;
  return {
    ...microbench('GetArticleView (cold)', () => {
      ct.invalidateCaches();
      ct.GetArticleView(selectionKey, selectionType);
    }, { iters }),
    articlesInView: len,
    pinnedCount: ct.pinnedArticleIds ? ct.pinnedArticleIds.size : null,
  };
}

/** Measures the other derived views a render touches. */
export function benchDerivedViews({ iters = 50 } = {}) {
  const ct = getContentTree();
  return [
    microbench('GetFolderFeedView (cold)', () => {
      ct.cachedFolderFeedView = null;
      ct.GetFolderFeedView();
    }, { iters }),
    microbench('GetFaviconMap (cold)', () => {
      ct.cachedFaviconMap = null;
      ct.GetFaviconMap();
    }, { iters }),
  ];
}

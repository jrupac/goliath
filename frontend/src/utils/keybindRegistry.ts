import { tinykeys } from 'tinykeys';

type KeyHandler = (event: KeyboardEvent) => void;
type KeyMap = Record<string, KeyHandler>;

interface KeyHistoryEntry {
  key: string;
  timestamp: number;
}

class KeybindRegistry {
  private keymaps: Map<string, KeyMap> = new Map();
  private unsubscribe: (() => void) | null = null;
  private history: KeyHistoryEntry[] = [];

  constructor() {
    if (typeof window !== 'undefined') {
      window.addEventListener('keydown', this.recordKeypress, true);
    }
  }

  private recordKeypress = (e: KeyboardEvent) => {
    const target = e.target as HTMLElement;
    if (
      target &&
      (target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable)
    ) {
      return;
    }

    this.history.push({
      key: e.key.toLowerCase(),
      timestamp: Date.now(),
    });

    if (this.history.length > 10) {
      this.history.shift();
    }
  };

  private hasRecentPrefixMatch(
    prefix: string[],
    suffixLength: number,
    currentTimestamp: number
  ): boolean {
    const L = prefix.length;
    const M = suffixLength;
    if (this.history.length < L + M) {
      return false;
    }

    // The prefix keys should be immediately preceding the suffix sequence in the history.
    const startIndex = this.history.length - M - L;

    // Verify each key in the prefix matches the history
    for (let i = 0; i < L; i++) {
      const histEntry = this.history[startIndex + i];
      if (histEntry.key !== prefix[i]) {
        return false;
      }
    }

    // Verify the timing sequence: the gap between consecutive keys must be < 1000ms
    for (let i = startIndex; i < this.history.length - 1; i++) {
      const currentEntry = this.history[i];
      const nextEntry = this.history[i + 1] || { timestamp: currentTimestamp };

      const gap = nextEntry.timestamp - currentEntry.timestamp;
      if (gap > 1000 || gap < 0) {
        return false;
      }
    }

    return true;
  }

  register(id: string, keymap: KeyMap) {
    this.keymaps.set(id, keymap);
    this.update();
  }

  unregister(id: string) {
    this.keymaps.delete(id);
    this.update();
  }

  private update() {
    if (this.unsubscribe) {
      this.unsubscribe();
      this.unsubscribe = null;
    }

    const combinedMap: KeyMap = {};
    for (const map of this.keymaps.values()) {
      Object.assign(combinedMap, map);
    }

    // Detect suffix collisions
    const collisions = new Map<string, string[][]>();
    const seqList = Object.keys(combinedMap);

    for (const S of seqList) {
      for (const T of seqList) {
        if (S === T) continue;
        const sParts = S.trim().toLowerCase().split(/\s+/);
        const tParts = T.trim().toLowerCase().split(/\s+/);

        if (sParts.length > tParts.length) {
          const isSuffix = tParts.every((part, idx) => {
            return part === sParts[sParts.length - tParts.length + idx];
          });

          if (isSuffix) {
            const prefix = sParts.slice(0, sParts.length - tParts.length);
            if (!collisions.has(T)) {
              collisions.set(T, []);
            }
            collisions.get(T)!.push(prefix);
          }
        }
      }
    }

    // Wrap handlers that have potential collision prefixes
    const finalMap: KeyMap = {};
    for (const [seq, originalHandler] of Object.entries(combinedMap)) {
      const prefixes = collisions.get(seq);
      if (prefixes && prefixes.length > 0) {
        const tParts = seq.trim().toLowerCase().split(/\s+/);
        const suffixLength = tParts.length;

        finalMap[seq] = (event: KeyboardEvent) => {
          const matchesCollidingPrefix = prefixes.some((prefix) => {
            return this.hasRecentPrefixMatch(prefix, suffixLength, Date.now());
          });
          if (matchesCollidingPrefix) {
            return; // Skip execution
          }
          originalHandler(event);
        };
      } else {
        finalMap[seq] = originalHandler;
      }
    }

    if (Object.keys(finalMap).length > 0) {
      this.unsubscribe = tinykeys(window, finalMap);
    }
  }
}

export const keybindRegistry = new KeybindRegistry();

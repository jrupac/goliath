import { tinykeys } from 'tinykeys';

type KeyHandler = (event: KeyboardEvent) => void;
type KeyMap = Record<string, KeyHandler>;

class KeybindRegistry {
  private keymaps: Map<string, KeyMap> = new Map();
  private unsubscribe: (() => void) | null = null;

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

    // Longest sequence first: only the first binding a press completes is
    // fired, so where one sequence ends with another -- "g s" against a bare
    // "s" -- the longer one has to be offered first or the press is taken as
    // the shorter one and the sequence never resolves. Offered in this order,
    // a bare "s" still falls through to its own binding, because it does not
    // match the opening press of "g s".
    const finalMap: KeyMap = Object.fromEntries(
      Object.entries(combinedMap).sort(
        (a, b) =>
          b[0].trim().split(/\s+/).length - a[0].trim().split(/\s+/).length
      )
    );

    if (Object.keys(finalMap).length > 0) {
      // Auto-repeat from a held key is ignored by default, but holding a
      // navigation key to walk down a long list is worth keeping. This filter
      // drops only what is never meant as a shortcut: a keystroke that belongs
      // to text composition, and one typed into a field rather than at the
      // listener itself.
      this.unsubscribe = tinykeys(window, finalMap, {
        ignore: (event: KeyboardEvent) => {
          if (event.isComposing) {
            return true;
          }
          const target = event.target;
          return (
            target !== event.currentTarget &&
            target instanceof Element &&
            target.matches('[contenteditable],input,select,textarea')
          );
        },
      });
    }
  }
}

export const keybindRegistry = new KeybindRegistry();

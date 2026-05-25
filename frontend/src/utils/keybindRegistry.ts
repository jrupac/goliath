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

    if (Object.keys(combinedMap).length > 0) {
      this.unsubscribe = tinykeys(window, combinedMap);
    }
  }
}

export const keybindRegistry = new KeybindRegistry();

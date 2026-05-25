declare module 'tinykeys' {
  export interface KeyBindingMap {
    [key: string]: (event: KeyboardEvent) => void;
  }

  export interface Options {
    event?: 'keydown' | 'keyup' | 'keypress';
    timeout?: number;
  }

  export function tinykeys(
    target: Window | HTMLElement | EventTarget,
    keymap: KeyBindingMap,
    options?: Options
  ): () => void;
}

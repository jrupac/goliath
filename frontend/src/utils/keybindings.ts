/** Exhaustive union of all valid handler keys — catches typos at compile time. */
export type HandlerKey =
  | 'toggleTheme'
  | 'toggleHideEmpty'
  | 'toggleKeybindingsModal'
  | 'scrollDown'
  | 'scrollUp'
  | 'openInTab'
  | 'togglePreviews'
  | 'toggleSmoothScroll'
  | 'goAll'
  | 'goUnread'
  | 'markAllRead'
  | 'toggleReaderMode';

/** Shape of a single keybinding entry. */
export interface Keybinding {
  key: string; // event.key value (e.g. "j", "ArrowDown")
  display: string[]; // each element renders as a separate key chip
  label: string; // short label (e.g. "Scroll down")
  description?: string; // longer description for modal (optional)
  handlerKey: HandlerKey; // typed key to look up handler in component's handler map
  isChord?: boolean; // true for any multi-key entry (excluded from direct lookup)
  isSequential?: boolean; // true for press-then-release sequences (g→a, g→u)
}

/** Pre-grouped keybinding definitions. */
export const Keybindings: {
  global: Keybinding[];
  articleList: Keybinding[];
  articleView: Keybinding[];
} = {
  global: [
    {
      key: '?',
      display: ['?'],
      label: 'Show keyboard shortcuts',
      description: 'Open this help dialog',
      handlerKey: 'toggleKeybindingsModal',
    },
    {
      key: 't',
      display: ['t'],
      label: 'Toggle theme',
      description: 'Switch between dark and default theme',
      handlerKey: 'toggleTheme',
    },
    {
      key: 'u',
      display: ['u'],
      label: 'Toggle showing empty feeds',
      description: 'Show/hide feeds with no unread items',
      handlerKey: 'toggleHideEmpty',
    },
  ],

  articleList: [
    {
      key: 'j',
      display: ['j', '↓'],
      label: 'Next article',
      description: 'Select the next article',
      handlerKey: 'scrollDown',
    },
    {
      key: 'ArrowDown',
      display: [],
      label: 'Next article',
      description: 'Select the next article',
      handlerKey: 'scrollDown',
    },
    {
      key: 'k',
      display: ['k', '↑'],
      label: 'Previous article',
      description: 'Select the previous article',
      handlerKey: 'scrollUp',
    },
    {
      key: 'ArrowUp',
      display: [],
      label: 'Previous article',
      description: 'Select the previous article',
      handlerKey: 'scrollUp',
    },
    {
      key: 'v',
      display: ['v'],
      label: 'Open in tab',
      description: 'Open the selected article in a new browser tab',
      handlerKey: 'openInTab',
    },
    {
      key: 'p',
      display: ['p'],
      label: 'Toggle previews',
      description: 'Show/hide article image previews in the list',
      handlerKey: 'togglePreviews',
    },
    {
      key: 'f',
      display: ['f'],
      label: 'Toggle smooth scroll',
      description: 'Enable/disable smooth scrolling animation',
      handlerKey: 'toggleSmoothScroll',
    },
    {
      key: 'a',
      display: ['g', 'a'],
      label: 'Go to All',
      description: 'Navigate to the All items stream',
      handlerKey: 'goAll',
      isChord: true,
      isSequential: true,
    },
    {
      key: 'u',
      display: ['g', 'u'],
      label: 'Go to Unread',
      description: 'Navigate to the Unread items stream',
      handlerKey: 'goUnread',
      isChord: true,
      isSequential: true,
    },
    {
      key: 'I',
      display: ['Shift', 'I'],
      label: 'Mark all as read',
      description: 'Mark all articles in the current view as read',
      handlerKey: 'markAllRead',
      isChord: true,
    },
  ],

  articleView: [
    {
      key: 'm',
      display: ['m'],
      label: 'Reader mode',
      description: 'Toggle readability-optimized article view',
      handlerKey: 'toggleReaderMode',
    },
  ],
};

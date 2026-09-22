import { JSDOM } from 'jsdom'

// Installs a fresh jsdom document as the global DOM for one test file.
// Views use plain global `document`/`confirm` like real browser code,
// so tests wire those globals up the same way a browser would.
export function setupDom() {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', {
    url: 'http://localhost/',
  })
  const g = globalThis as Record<string, unknown>
  g.window = dom.window
  g.document = dom.window.document
  g.localStorage = dom.window.localStorage
  g.HTMLElement = dom.window.HTMLElement
  g.Event = dom.window.Event
  g.confirm = () => true
  return dom
}

// Lets pending promise chains (api stub resolutions, then draw()) run
// before assertions look at the DOM.
export function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

// Dispatches a real Event through jsdom's listener machinery, rather
// than calling handlers directly, so tests exercise the same path a
// browser click or submit would.
export function fire(el: EventTarget, type: string) {
  el.dispatchEvent(new (globalThis as { Event: typeof Event }).Event(type, { bubbles: true, cancelable: true }))
}

import { computed, effect, Injectable, signal, Signal } from '@angular/core';

/** The two appearances the interface has. */
export type ThemeMode = 'light' | 'dark';

/**
 * Where a deliberate choice is kept.
 *
 * Namespaced because a desktop webview and a container dashboard can end up
 * sharing an origin with something else, and a bare "theme" key is the sort
 * of thing two applications silently fight over.
 */
export const THEME_STORAGE_KEY = 'trawl.theme';

/**
 * Decides which appearance the interface wears.
 *
 * The rule is: follow the environment until the operator says otherwise, then
 * follow the operator — permanently, across sessions.
 *
 * Both halves matter. An application that ignores the system appearance
 * announces that its preferences outrank the user's, and on a machine set to
 * dark for a reason — a night shift, a light-sensitive condition — that is not
 * a cosmetic complaint. An application that overrides an explicit choice on
 * every launch is worse: the operator has already told it what they want and
 * it keeps forgetting, which is the kind of small, repeated insult that makes
 * a tool feel unreliable at everything else too.
 *
 * Where the environment expresses no preference at all, the answer is light.
 * That is a deliberate default rather than an arbitrary one: this interface is
 * read in daylight, in meetings, and on projectors, and light is the appearance
 * that survives all three.
 */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  /**
   * The operator's explicit choice, or null while they have not made one.
   *
   * Null is a distinct state rather than "light", and the distinction is the
   * whole feature: a user who has never chosen must track their system when it
   * changes, and a user who has chosen must not.
   */
  private readonly chosen = signal<ThemeMode | null>(this.readChoice());

  /** What the environment currently asks for. */
  private readonly environment = signal<ThemeMode>(this.readEnvironment());

  /** The appearance in force. */
  readonly theme: Signal<ThemeMode> = computed(() => this.chosen() ?? this.environment());

  /** True while no explicit choice has been made and the system is followed. */
  readonly followsEnvironment = computed(() => this.chosen() === null);

  constructor() {
    this.watchEnvironment();

    // The document is told as well as the templates. Tailwind classes cover
    // what this application draws; `color-scheme` covers what it does not —
    // scrollbars, form controls, the flash of white before Angular boots —
    // and those reading dark against a light interface is exactly the sort of
    // seam that makes an application feel unfinished.
    effect(() => {
      const mode = this.theme();
      const root = typeof document === 'undefined' ? null : document.documentElement;
      if (!root) {
        return;
      }
      root.classList.toggle('dark', mode === 'dark');
      root.style.colorScheme = mode;
    });
  }

  /** Records a deliberate choice, which outlives the session. */
  set(mode: ThemeMode): void {
    this.chosen.set(mode);
    this.writeChoice(mode);
  }

  toggle(): void {
    this.set(this.theme() === 'dark' ? 'light' : 'dark');
  }

  /**
   * Forgets the choice and returns to following the environment.
   *
   * Offered because a preference that cannot be withdrawn is a trap: without
   * this, a single click on the toggle would pin the appearance for good, and
   * the only way back would be to clear browser storage.
   */
  followEnvironment(): void {
    this.chosen.set(null);
    this.clearChoice();
  }

  // ─── The environment ───────────────────────────────────────────────────────

  private mediaQuery(): MediaQueryList | null {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return null;
    }
    return window.matchMedia('(prefers-color-scheme: dark)');
  }

  /**
   * Light unless the environment actively asks for dark.
   *
   * "No preference" and "unable to ask" both resolve to light rather than to
   * a guess.
   */
  private readEnvironment(): ThemeMode {
    return this.mediaQuery()?.matches ? 'dark' : 'light';
  }

  /**
   * Tracks the environment for as long as the application runs.
   *
   * A system that switches at sunset should carry the interface with it, for a
   * user who has not overridden it. Reading the preference once at startup
   * would follow the environment only in the sense that a stopped clock
   * follows the time.
   */
  private watchEnvironment(): void {
    const query = this.mediaQuery();
    if (!query || typeof query.addEventListener !== 'function') {
      return;
    }
    query.addEventListener('change', event => this.environment.set(event.matches ? 'dark' : 'light'));
  }

  // ─── Persistence ───────────────────────────────────────────────────────────

  /**
   * Storage is reached through try/catch throughout.
   *
   * It is genuinely absent or forbidden in more places than it looks: a
   * `file://` webview, a container served in private browsing, a locked-down
   * corporate profile. None of those are reasons for the interface to fail to
   * render, so a storage failure costs the persistence and nothing else — the
   * choice still holds for the session, in memory.
   */
  private readChoice(): ThemeMode | null {
    try {
      const stored = localStorage.getItem(THEME_STORAGE_KEY);
      return stored === 'light' || stored === 'dark' ? stored : null;
    } catch {
      return null;
    }
  }

  private writeChoice(mode: ThemeMode): void {
    try {
      localStorage.setItem(THEME_STORAGE_KEY, mode);
    } catch {
      // Deliberately silent: the operator asked for an appearance, not for a
      // report on the storage layer.
    }
  }

  private clearChoice(): void {
    try {
      localStorage.removeItem(THEME_STORAGE_KEY);
    } catch {
      // As above.
    }
  }
}

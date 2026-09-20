import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';

import { THEME_STORAGE_KEY, ThemeMode, ThemeService } from './theme.service';

/**
 * A stand-in for the media query the browser answers with the operating
 * system's appearance, with a handle on the listener so a test can make the
 * environment change mid-session.
 */
const stubEnvironment = (prefersDark: boolean) => {
  const listeners: Array<(e: { matches: boolean }) => void> = [];
  const query = {
    matches: prefersDark,
    addEventListener: (_: string, listener: (e: { matches: boolean }) => void) => {
      listeners.push(listener);
    },
    removeEventListener: () => {}
  };
  vi.stubGlobal('matchMedia', (q: string) => {
    // Only the colour-scheme question is answered; anything else is somebody
    // else's query and must not be given this answer by accident.
    expect(q).toBe('(prefers-color-scheme: dark)');
    return query as unknown as MediaQueryList;
  });
  return {
    change: (dark: boolean) => listeners.forEach(l => l({ matches: dark }))
  };
};

/** The service is constructed lazily so each test can set the world up first. */
const serviceWith = (): ThemeService => TestBed.inject(ThemeService);

describe('ThemeService', () => {
  beforeEach(() => {
    localStorage.clear();
    TestBed.resetTestingModule();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  describe('following the environment', () => {
    // The interface is read in daylight, in meetings and on projectors.
    // Light is the appearance that survives all three, so it is what an
    // environment expressing no preference resolves to.
    it('is light when the environment asks for nothing in particular', () => {
      stubEnvironment(false);
      expect(serviceWith().theme()).toBe('light');
    });

    it('is light when the environment cannot be asked at all', () => {
      vi.stubGlobal('matchMedia', undefined);
      expect(serviceWith().theme()).toBe('light');
    });

    // A machine set to dark is often set that way for a reason — a night
    // shift, a light-sensitive condition — and overriding it says the
    // application's preferences outrank the operator's.
    it('is dark when the system is', () => {
      stubEnvironment(true);
      expect(serviceWith().theme()).toBe('dark');
    });

    // Reading the preference once at startup would follow the environment
    // only in the sense that a stopped clock follows the time.
    it('follows the system when it changes mid-session', () => {
      const env = stubEnvironment(false);
      const themes = serviceWith();

      env.change(true);
      expect(themes.theme()).toBe('dark');

      env.change(false);
      expect(themes.theme()).toBe('light');
    });
  });

  describe('a deliberate choice', () => {
    it('overrides the environment', () => {
      stubEnvironment(true);
      const themes = serviceWith();

      themes.set('light');
      expect(themes.theme()).toBe('light');
      expect(themes.followsEnvironment()).toBe(false);
    });

    // The whole point of recording it. An application that forgets an explicit
    // choice on every launch is one the operator has to correct repeatedly,
    // which is the kind of small repeated insult that makes a tool feel
    // unreliable at everything else too.
    it('survives into the next session', () => {
      stubEnvironment(true);
      serviceWith().set('light');

      TestBed.resetTestingModule();
      stubEnvironment(true);
      expect(serviceWith().theme()).toBe('light');
    });

    // Once the operator has spoken, the system no longer speaks for them.
    it('is not overridden when the system later changes', () => {
      const env = stubEnvironment(false);
      const themes = serviceWith();
      themes.set('dark');

      env.change(false);
      expect(themes.theme()).toBe('dark');
    });

    it('toggles between the two appearances', () => {
      stubEnvironment(false);
      const themes = serviceWith();

      themes.toggle();
      expect(themes.theme()).toBe('dark');
      themes.toggle();
      expect(themes.theme()).toBe('light');
      expect(themes.followsEnvironment()).toBe(false);
    });

    // A preference that cannot be withdrawn is a trap: one click would
    // otherwise pin the appearance for good.
    it('can be withdrawn, restoring the system appearance', () => {
      const env = stubEnvironment(true);
      const themes = serviceWith();
      themes.set('light');

      themes.followEnvironment();

      expect(themes.followsEnvironment()).toBe(true);
      expect(themes.theme()).toBe('dark');
      expect(localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();

      env.change(false);
      expect(themes.theme()).toBe('light');
    });

    it('ignores a stored value it does not recognise', () => {
      localStorage.setItem(THEME_STORAGE_KEY, 'sepia');
      stubEnvironment(true);

      const themes = serviceWith();
      expect(themes.followsEnvironment()).toBe(true);
      expect(themes.theme()).toBe('dark');
    });
  });

  describe('when storage is unavailable', () => {
    /**
     * A `file://` webview, private browsing and a locked-down corporate
     * profile all refuse storage. None of them is a reason for the interface
     * to stop working, so the failure costs the persistence and nothing else.
     */
    const refuseStorage = () => {
      vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
        throw new Error('storage is not available');
      });
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('storage is not available');
      });
    };

    afterEach(() => vi.restoreAllMocks());

    it('still starts from the environment', () => {
      refuseStorage();
      stubEnvironment(true);
      expect(serviceWith().theme()).toBe('dark');
    });

    it('still honours the choice for the session', () => {
      refuseStorage();
      stubEnvironment(true);
      const themes = serviceWith();

      expect(() => themes.set('light')).not.toThrow();
      expect(themes.theme()).toBe('light');
    });
  });

  describe('the document', () => {
    // Tailwind classes cover what this application draws; `color-scheme`
    // covers what it does not — scrollbars, form controls, the flash before
    // Angular boots — and those reading dark against a light interface is the
    // sort of seam that makes an application feel unfinished.
    it('is told which appearance is in force', () => {
      stubEnvironment(false);
      const themes = serviceWith();
      TestBed.tick();

      expect(document.documentElement.style.colorScheme).toBe('light');
      expect(document.documentElement.classList.contains('dark')).toBe(false);

      themes.set('dark');
      TestBed.tick();

      expect(document.documentElement.style.colorScheme).toBe('dark');
      expect(document.documentElement.classList.contains('dark')).toBe(true);
    });
  });

  describe('the type', () => {
    it('admits exactly the two appearances', () => {
      const modes: ThemeMode[] = ['light', 'dark'];
      expect(modes.length).toBe(2);
    });
  });
});

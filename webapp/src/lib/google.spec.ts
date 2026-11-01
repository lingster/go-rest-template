import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
	BUTTON_MAX_WIDTH,
	BUTTON_MIN_WIDTH,
	GSI_CLIENT_SRC,
	buttonOptions,
	buttonWidth,
	credentialFrom,
	loadGoogleIdentity,
	resetGoogleIdentityLoader
} from './google.ts';
import { GSI_CSP, withGoogleCsp } from './googleCsp.ts';

describe('withGoogleCsp', () => {
	const base = { 'script-src': ['self'], 'style-src': ['self'], 'default-src': ['self'] };

	it('leaves the policy untouched when disabled', () => {
		expect(withGoogleCsp(base, false)).toBe(base);
	});

	it('adds only the GIS allowances when enabled, without unsafe-inline', () => {
		const out = withGoogleCsp({ ...base, 'connect-src': ['self'] }, true);
		expect(out['script-src']).toEqual(['self', 'https://accounts.google.com/gsi/client']);
		expect(out['style-src']).toEqual(['self', 'https://accounts.google.com/gsi/style']);
		expect(out['connect-src']).toEqual(['self', 'https://accounts.google.com/gsi/']);
		expect(out['frame-src']).toEqual(['https://accounts.google.com/gsi/']);
		expect(out['default-src']).toEqual(['self']);
		expect(JSON.stringify(out)).not.toContain('unsafe-inline');
		expect(Object.keys(GSI_CSP).sort()).toEqual([
			'connect-src',
			'frame-src',
			'script-src',
			'style-src'
		]);
	});

	it('does not mutate its input', () => {
		withGoogleCsp(base, true);
		expect(base['script-src']).toEqual(['self']);
	});
});

describe('buttonWidth / buttonOptions', () => {
	it('clamps to the range GIS accepts', () => {
		expect(buttonWidth(120)).toBe(BUTTON_MIN_WIDTH);
		expect(buttonWidth(320.4)).toBe(320);
		expect(buttonWidth(900)).toBe(BUTTON_MAX_WIDTH);
	});

	it('falls back to the max width for unknown sizes', () => {
		expect(buttonWidth(0)).toBe(BUTTON_MAX_WIDTH);
		expect(buttonWidth(Number.NaN)).toBe(BUTTON_MAX_WIDTH);
	});

	it('renders a "continue with" button', () => {
		expect(buttonOptions(300)).toMatchObject({ text: 'continue_with', width: 300 });
	});
});

describe('credentialFrom', () => {
	it('extracts the id token', () => {
		expect(credentialFrom({ credential: ' abc.def.ghi ' })).toBe('abc.def.ghi');
	});

	it('returns null when there is no credential', () => {
		expect(credentialFrom(undefined)).toBeNull();
		expect(credentialFrom({})).toBeNull();
		expect(credentialFrom({ credential: '  ' })).toBeNull();
	});
});

describe('loadGoogleIdentity', () => {
	beforeEach(() => resetGoogleIdentityLoader());

	function fakeDocument() {
		const listeners: Record<string, () => void> = {};
		const script = {
			src: '',
			async: false,
			defer: false,
			addEventListener: (type: string, fn: () => void) => (listeners[type] = fn)
		};
		const append = vi.fn();
		const doc = {
			createElement: vi.fn(() => script),
			head: { appendChild: append }
		} as unknown as Document;
		return { doc, script, listeners, append };
	}

	it('injects the GIS script once and resolves with google.accounts.id', async () => {
		const id = { initialize: vi.fn(), renderButton: vi.fn() };
		const win: { google?: { accounts: { id: typeof id } } } = {};
		const { doc, script, listeners, append } = fakeDocument();

		const first = loadGoogleIdentity(doc, win);
		const second = loadGoogleIdentity(doc, win);
		expect(append).toHaveBeenCalledTimes(1);
		expect(script.src).toBe(GSI_CLIENT_SRC);

		win.google = { accounts: { id } };
		listeners.load();
		expect(await first).toBe(id);
		expect(await second).toBe(id);
	});

	it('rejects (and can retry) when the script is blocked', async () => {
		const { doc, listeners } = fakeDocument();
		const p = loadGoogleIdentity(doc, {});
		listeners.error();
		await expect(p).rejects.toThrow('Could not load Google sign-in');

		// a failed load is not cached: the next call injects the script again
		const again = fakeDocument();
		void loadGoogleIdentity(again.doc, {}).catch(() => undefined);
		expect(again.append).toHaveBeenCalledTimes(1);
	});

	it('reuses an already loaded global without touching the DOM', async () => {
		const id = { initialize: vi.fn(), renderButton: vi.fn() };
		const { doc, append } = fakeDocument();
		expect(await loadGoogleIdentity(doc, { google: { accounts: { id } } })).toBe(id);
		expect(append).not.toHaveBeenCalled();
	});
});

/** Google Identity Services (GIS) helpers: script loading, CSP allowances and pure option logic. */

export const GSI_CLIENT_SRC = 'https://accounts.google.com/gsi/client';

export const BUTTON_MIN_WIDTH = 200;
export const BUTTON_MAX_WIDTH = 400;

/** GIS only accepts button widths between 200 and 400 px. */
export function buttonWidth(containerWidth: number): number {
	if (!Number.isFinite(containerWidth) || containerWidth <= 0) return BUTTON_MAX_WIDTH;
	return Math.round(Math.min(BUTTON_MAX_WIDTH, Math.max(BUTTON_MIN_WIDTH, containerWidth)));
}

export interface GoogleCredentialResponse {
	credential?: string;
	select_by?: string;
}

/** The ID token from a GIS callback, or null when the user dismissed the flow. */
export function credentialFrom(res: GoogleCredentialResponse | undefined): string | null {
	const token = res?.credential?.trim();
	return token ? token : null;
}

export interface GoogleAccountsId {
	initialize(config: {
		client_id: string;
		callback: (res: GoogleCredentialResponse) => void;
		ux_mode?: 'popup' | 'redirect';
		auto_select?: boolean;
	}): void;
	renderButton(parent: HTMLElement, options: Record<string, unknown>): void;
}

interface GoogleGlobal {
	google?: { accounts?: { id?: GoogleAccountsId } };
}

let loading: Promise<GoogleAccountsId> | null = null;

/** Load the GIS client script once and resolve with `google.accounts.id`. Rejects if it is blocked or offline. */
export function loadGoogleIdentity(
	doc: Document = document,
	win: GoogleGlobal = window as unknown as GoogleGlobal
): Promise<GoogleAccountsId> {
	loading ??= new Promise<GoogleAccountsId>((resolve, reject) => {
		const fail = () => {
			loading = null; // allow a retry
			reject(new Error('Could not load Google sign-in'));
		};
		const ready = () => {
			const id = win.google?.accounts?.id;
			if (id) resolve(id);
			else fail();
		};
		if (win.google?.accounts?.id) return ready();
		const script = doc.createElement('script');
		script.src = GSI_CLIENT_SRC;
		script.async = true;
		script.defer = true;
		script.addEventListener('load', ready);
		script.addEventListener('error', fail);
		doc.head.appendChild(script);
	});
	return loading;
}

/** Test hook: forget a cached load. */
export function resetGoogleIdentityLoader(): void {
	loading = null;
}

/** Options for the rendered "Continue with Google" button. */
export function buttonOptions(containerWidth: number): Record<string, unknown> {
	return {
		type: 'standard',
		theme: 'outline',
		size: 'large',
		text: 'continue_with',
		shape: 'rectangular',
		logo_alignment: 'left',
		width: buttonWidth(containerWidth)
	};
}

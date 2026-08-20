let redirectingForAuthentication = false;

const jsonHeaders = { Accept: "application/json" };

async function authenticationErrorCode(response) {
  try {
    const body = await response.clone().json();
    return body.error?.code ?? "";
  } catch {
    return "";
  }
}

/**
 * Reads the human-readable message from a Kaeru API error body, falling back to
 * a caller-supplied message when the body is missing or malformed.
 */
export async function apiErrorMessage(response, fallback) {
  try {
    const body = await response.json();
    return body.error?.message || fallback;
  } catch {
    return fallback;
  }
}

/**
 * Wraps fetch so an expired session or revoked administrator access logs the
 * user out and returns them to the login page instead of surfacing a raw error.
 */
export async function apiFetch(input, init) {
  const response = await window.fetch(input, init);
  if (redirectingForAuthentication || (response.status !== 401 && response.status !== 403)) {
    return response;
  }

  const errorCode = await authenticationErrorCode(response);
  const authenticationRequired = response.status === 401
    && errorCode === "authentication_required";
  const administratorRequired = response.status === 403
    && errorCode === "administrator_required";
  if (!authenticationRequired && !administratorRequired) {
    return response;
  }

  redirectingForAuthentication = true;
  try {
    await window.fetch("/api/v1/session/logout", {
      method: "POST",
      headers: jsonHeaders,
    });
  } finally {
    const message = administratorRequired
      ? "Your account no longer has Kaeru Core administrator access. You have been logged out."
      : "Your session has expired. Please log in again.";
    const query = new URLSearchParams({ error: message });
    window.location.assign(`/login?${query.toString()}`);
  }

  return response;
}

/** Fetches JSON from an authenticated endpoint, throwing on a non-2xx response. */
export async function fetchJSON(path, fallbackMessage, init = {}) {
  const response = await apiFetch(path, { ...init, headers: { ...jsonHeaders, ...init.headers } });
  if (!response.ok) {
    throw new Error(await apiErrorMessage(response, fallbackMessage));
  }
  return response.json();
}

/** Sends a JSON body and decodes the JSON response. */
export function sendJSON(path, method, body, fallbackMessage) {
  return fetchJSON(path, fallbackMessage, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Sends multipart form data and decodes the JSON response. */
export function sendForm(path, method, form, fallbackMessage) {
  return fetchJSON(path, fallbackMessage, { method, body: form });
}

/**
 * Fetches a public endpoint that must work before a session exists, such as
 * health, setup status, and OIDC branding.
 */
export async function fetchPublicJSON(path) {
  const response = await window.fetch(path, { headers: jsonHeaders });
  if (!response.ok) {
    throw new Error(`Request to ${path} failed`);
  }
  return response.json();
}

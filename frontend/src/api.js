let redirectingForAuthentication = false;

async function authenticationErrorCode(response) {
  try {
    const body = await response.clone().json();
    return body.error?.code ?? "";
  } catch {
    return "";
  }
}

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
      headers: { Accept: "application/json" },
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

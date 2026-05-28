const JSON_HEADERS = {
  "Content-Type": "application/json; charset=utf-8",
  "Cache-Control": "no-store",
};

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (url.pathname !== canaryPath(env)) {
      return notFound();
    }
    if (!methodAllowed(request.method)) {
      return new Response(null, {
        status: 405,
        headers: { Allow: "GET, HEAD", ...JSON_HEADERS },
      });
    }
    if (!tokenAccepted(request, env)) {
      return notFound();
    }
    if (request.method === "HEAD") {
      return new Response(null, { status: 204, headers: JSON_HEADERS });
    }
    return Response.json({ ok: true }, { headers: JSON_HEADERS });
  },
};

function methodAllowed(method) {
  return method === "GET" || method === "HEAD";
}

function canaryPath(env) {
  const value = String(env.CANARY_PATH || "/edge-canary").trim();
  return value.startsWith("/") ? value : `/${value}`;
}

function tokenAccepted(request, env) {
  const token = String(env.CANARY_TOKEN || "").trim();
  if (!token) {
    return true;
  }
  return request.headers.get("x-canary-token") === token;
}

function notFound() {
  return Response.json({ ok: false }, { status: 404, headers: JSON_HEADERS });
}

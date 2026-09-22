// Throwaway local app for exercising the inspector at 127.0.0.1:4040.
//
//   node server/dashboard/testserver.mjs      # listens on 8080
//   goport http 8080                          # tunnel + inspector
//
// Echoes the request back as JSON so the inspector has real headers, bodies
// and status codes to render. Routes:
//
//   /                 200, small JSON
//   /api/users        200, larger JSON (tests the scroll pane)
//   /slow             200 after ~1.2s (tests the duration colour)
//   /boom             500 (tests the status badge)
//   /notfound         404
//   /xss              200, body containing an injection payload -- it must
//                     render as visible text, never execute
import { createServer } from "node:http";

const PORT = Number(process.argv[2] || 8080);

const read = (req) =>
  new Promise((resolve) => {
    let data = "";
    req.on("data", (c) => (data += c));
    req.on("end", () => resolve(data));
  });

const send = (res, code, payload) => {
  const body = JSON.stringify(payload, null, 2);
  res.writeHead(code, {
    "Content-Type": "application/json",
    "X-Test-Server": "goport-inspector-fixture",
  });
  res.end(body);
};

createServer(async (req, res) => {
  const body = await read(req);
  const url = new URL(req.url, `http://localhost:${PORT}`);
  const echo = { method: req.method, path: url.pathname, headers: req.headers, body: body || null };

  switch (url.pathname) {
    case "/boom":
      return send(res, 500, { error: "intentional failure", ...echo });
    case "/notfound":
      return send(res, 404, { error: "no such thing", ...echo });
    case "/slow":
      await new Promise((r) => setTimeout(r, 1200));
      return send(res, 200, { slow: true, ...echo });
    case "/xss":
      // If the inspector is fixed, this shows as text. If it ever regresses,
      // the alert fires inside the inspector.
      return send(res, 200, {
        note: "must render as plain text",
        payload: "<img src=x onerror=alert('inspector-xss')>",
        second: "</script><svg onload=alert(1)>",
        ...echo,
      });
    case "/api/users":
      return send(res, 200, {
        users: Array.from({ length: 40 }, (_, i) => ({
          id: i + 1,
          name: `User ${i + 1}`,
          email: `user${i + 1}@example.com`,
          active: i % 3 !== 0,
          score: Number((Math.sin(i) * 50 + 50).toFixed(2)),
          tags: ["alpha", "beta", "gamma"].slice(0, (i % 3) + 1),
          meta: i % 5 === 0 ? null : { seen: `2026-09-${String((i % 28) + 1).padStart(2, "0")}` },
        })),
        ...echo,
      });
    default:
      return send(res, 200, { ok: true, ...echo });
  }
}).listen(PORT, "127.0.0.1", () => {
  console.log(`test server listening on http://127.0.0.1:${PORT}`);
  console.log(`now run:  goport http ${PORT}`);
});

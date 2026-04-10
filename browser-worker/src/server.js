const { chromium } = require("playwright");
const http = require("http");
const { URL } = require("url");

const PORT = parseInt(process.env.PORT || "9222", 10);
const MAX_SESSIONS = parseInt(process.env.MAX_SESSIONS || "5", 10);
const SESSION_TIMEOUT_MS = parseInt(process.env.SESSION_TIMEOUT_MS || "1800000", 10);
const IDLE_TIMEOUT_MS = parseInt(process.env.IDLE_TIMEOUT_MS || "600000", 10);

const sessions = new Map();
let browser = null;

async function ensureBrowser() {
  if (!browser || !browser.isConnected()) {
    browser = await chromium.launch({
      headless: true,
      args: [
        "--no-sandbox",
        "--disable-setuid-sandbox",
        "--disable-dev-shm-usage",
        "--disable-gpu",
        "--disable-extensions",
      ],
    });
  }
  return browser;
}

async function acquireSession(agentId, taskId) {
  const activeCount = [...sessions.values()].filter(
    (s) => s.state === "active"
  ).length;
  if (activeCount >= MAX_SESSIONS) {
    throw new Error(
      `Browser pool full (${activeCount}/${MAX_SESSIONS} sessions)`
    );
  }

  const b = await ensureBrowser();
  const context = await b.newContext({
    viewport: { width: 1280, height: 720 },
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();

  const sessionId = `bs-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const session = {
    id: sessionId,
    agentId,
    taskId,
    state: "active",
    context,
    page,
    createdAt: new Date(),
    lastUsedAt: new Date(),
  };

  sessions.set(sessionId, session);
  return session;
}

async function releaseSession(sessionId) {
  const session = sessions.get(sessionId);
  if (!session) return;

  try {
    await session.context.close();
  } catch {
    // Context already closed.
  }
  session.state = "closed";
  sessions.delete(sessionId);
}

async function takeScreenshot(sessionId) {
  const session = sessions.get(sessionId);
  if (!session || session.state !== "active") {
    throw new Error(`Session ${sessionId} not found or inactive`);
  }
  session.lastUsedAt = new Date();
  const buffer = await session.page.screenshot({ type: "png", fullPage: false });
  return buffer.toString("base64");
}

async function navigateTo(sessionId, url) {
  const session = sessions.get(sessionId);
  if (!session || session.state !== "active") {
    throw new Error(`Session ${sessionId} not found or inactive`);
  }
  session.lastUsedAt = new Date();
  const response = await session.page.goto(url, {
    waitUntil: "domcontentloaded",
    timeout: 30000,
  });
  return {
    url: session.page.url(),
    title: await session.page.title(),
    statusCode: response ? response.status() : 0,
  };
}

async function getDOMSnapshot(sessionId) {
  const session = sessions.get(sessionId);
  if (!session || session.state !== "active") {
    throw new Error(`Session ${sessionId} not found or inactive`);
  }
  session.lastUsedAt = new Date();
  return {
    html: await session.page.content(),
    text: await session.page.evaluate(() => document.body.innerText),
    url: session.page.url(),
    title: await session.page.title(),
  };
}

async function clickElement(sessionId, selector) {
  const session = sessions.get(sessionId);
  if (!session || session.state !== "active") {
    throw new Error(`Session ${sessionId} not found or inactive`);
  }
  session.lastUsedAt = new Date();
  await session.page.click(selector, { timeout: 10000 });
}

async function typeInto(sessionId, selector, text) {
  const session = sessions.get(sessionId);
  if (!session || session.state !== "active") {
    throw new Error(`Session ${sessionId} not found or inactive`);
  }
  session.lastUsedAt = new Date();
  await session.page.fill(selector, text);
}

function listSessions() {
  return [...sessions.values()].map((s) => ({
    id: s.id,
    agentId: s.agentId,
    taskId: s.taskId,
    state: s.state,
    createdAt: s.createdAt.toISOString(),
    lastUsedAt: s.lastUsedAt.toISOString(),
  }));
}

function reapExpired() {
  const now = Date.now();
  let reaped = 0;
  for (const [id, session] of sessions) {
    const age = now - session.createdAt.getTime();
    const idle = now - session.lastUsedAt.getTime();
    if (age > SESSION_TIMEOUT_MS || idle > IDLE_TIMEOUT_MS) {
      releaseSession(id);
      reaped++;
    }
  }
  return reaped;
}

setInterval(reapExpired, 60000);

function sendJSON(res, statusCode, data) {
  res.writeHead(statusCode, { "Content-Type": "application/json" });
  res.end(JSON.stringify(data));
}

async function handleRequest(req, res) {
  const parsedUrl = new URL(req.url, `http://localhost:${PORT}`);
  const path = parsedUrl.pathname;

  if (req.method === "GET" && path === "/health") {
    return sendJSON(res, 200, {
      status: "healthy",
      sessions: sessions.size,
      maxSessions: MAX_SESSIONS,
    });
  }

  if (req.method === "GET" && path === "/sessions") {
    return sendJSON(res, 200, listSessions());
  }

  if (req.method === "POST" && path === "/sessions") {
    const body = await readBody(req);
    const { agentId, taskId } = JSON.parse(body);
    try {
      const session = await acquireSession(agentId || "", taskId || "");
      return sendJSON(res, 201, {
        id: session.id,
        state: session.state,
        createdAt: session.createdAt.toISOString(),
      });
    } catch (err) {
      return sendJSON(res, 429, { error: err.message });
    }
  }

  if (req.method === "DELETE" && path.startsWith("/sessions/")) {
    const sessionId = path.split("/")[2];
    await releaseSession(sessionId);
    return sendJSON(res, 200, { released: sessionId });
  }

  if (req.method === "POST" && path.match(/^\/sessions\/[^/]+\/screenshot$/)) {
    const sessionId = path.split("/")[2];
    try {
      const base64 = await takeScreenshot(sessionId);
      return sendJSON(res, 200, {
        data: base64,
        mimeType: "image/png",
      });
    } catch (err) {
      return sendJSON(res, 400, { error: err.message });
    }
  }

  if (req.method === "POST" && path.match(/^\/sessions\/[^/]+\/navigate$/)) {
    const sessionId = path.split("/")[2];
    const body = await readBody(req);
    const { url } = JSON.parse(body);
    try {
      const result = await navigateTo(sessionId, url);
      return sendJSON(res, 200, result);
    } catch (err) {
      return sendJSON(res, 400, { error: err.message });
    }
  }

  if (req.method === "POST" && path.match(/^\/sessions\/[^/]+\/dom$/)) {
    const sessionId = path.split("/")[2];
    try {
      const snapshot = await getDOMSnapshot(sessionId);
      return sendJSON(res, 200, snapshot);
    } catch (err) {
      return sendJSON(res, 400, { error: err.message });
    }
  }

  if (req.method === "POST" && path.match(/^\/sessions\/[^/]+\/click$/)) {
    const sessionId = path.split("/")[2];
    const body = await readBody(req);
    const { selector } = JSON.parse(body);
    try {
      await clickElement(sessionId, selector);
      return sendJSON(res, 200, { clicked: selector });
    } catch (err) {
      return sendJSON(res, 400, { error: err.message });
    }
  }

  if (req.method === "POST" && path.match(/^\/sessions\/[^/]+\/type$/)) {
    const sessionId = path.split("/")[2];
    const body = await readBody(req);
    const { selector, text } = JSON.parse(body);
    try {
      await typeInto(sessionId, selector, text);
      return sendJSON(res, 200, { typed: text });
    } catch (err) {
      return sendJSON(res, 400, { error: err.message });
    }
  }

  sendJSON(res, 404, { error: "not found" });
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => resolve(Buffer.concat(chunks).toString()));
    req.on("error", reject);
  });
}

const server = http.createServer(handleRequest);

server.listen(PORT, () => {
  console.log(`zclaw-browser-worker listening on port ${PORT}`);
  console.log(`max sessions: ${MAX_SESSIONS}`);
  console.log(`session timeout: ${SESSION_TIMEOUT_MS}ms`);
  console.log(`idle timeout: ${IDLE_TIMEOUT_MS}ms`);
});

process.on("SIGTERM", async () => {
  console.log("shutting down...");
  for (const [id] of sessions) {
    await releaseSession(id);
  }
  if (browser) {
    await browser.close();
  }
  server.close();
  process.exit(0);
});

#!/usr/bin/env node

import { createHash, randomUUID, sign } from "node:crypto";
import { readFileSync } from "node:fs";

function fail(message) {
  process.stderr.write(`${message}\n`);
  process.exit(1);
}

function option(name) {
  const index = process.argv.indexOf(name);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

const method = option("--method")?.toUpperCase();
const target = option("--target");
const bodyFile = option("--body-file");
const timestamp = option("--timestamp") ?? String(Math.floor(Date.now() / 1000));
const nonce = option("--nonce") ?? randomUUID();
const explicitIdempotencyKey = option("--idempotency-key");
const keyId = process.env.BROSETTLEMENT_API_KEY_ID;
const privateKeyFile = process.env.BROSETTLEMENT_API_PRIVATE_KEY_FILE;
const privateKeyInline = process.env.BROSETTLEMENT_API_PRIVATE_KEY;

if (!method || !target) {
  fail("Usage: sign-request.mjs --method METHOD --target /exact/path?query [--body-file FILE] [--idempotency-key VALUE] [--timestamp UNIX_SECONDS] [--nonce VALUE]");
}
if (!target.startsWith("/")) fail("--target must be the exact request target beginning with /");
if (!keyId) fail("Set BROSETTLEMENT_API_KEY_ID");
if (!privateKeyFile && !privateKeyInline) {
  fail("Set BROSETTLEMENT_API_PRIVATE_KEY_FILE or BROSETTLEMENT_API_PRIVATE_KEY");
}
if (privateKeyFile && privateKeyInline) {
  fail("Set only one private-key source");
}
if (!/^(?:0|[1-9][0-9]*)$/.test(timestamp)) fail("Timestamp must be unsigned Unix seconds");
if (!/^[A-Za-z0-9._~-]{16,128}$/.test(nonce)) fail("Nonce must be 16-128 allowed characters");

function requiresBodyHash(requestMethod, requestTarget) {
  if (requestMethod !== "POST") return false;
  const path = requestTarget.split("?", 1)[0];
  if ([
    "/api/v1/wallets",
    "/api/v1/ledger/accounts",
    "/api/v1/transactions",
  ].includes(path)) {
    return true;
  }
  if (path.startsWith("/api/v1/co-signer/intents/") && path.endsWith("/result")) {
    return true;
  }
  return path.startsWith("/api/v1/co-signer/sessions/") && path.endsWith("/messages");
}

function requiresExplicitEmptyFormBody(requestMethod, requestTarget) {
  return requestMethod === "POST" &&
    requestTarget.split("?", 1)[0] === "/api/v1/mpc/initialize";
}

function requiresIdempotency(requestMethod, requestTarget) {
  if (requestMethod !== "POST") return false;
  const path = requestTarget.split("?", 1)[0];
  if ([
    "/api/v1/mpc/initialize",
    "/api/v1/wallets",
    "/api/v1/transactions",
  ].includes(path)) {
    return true;
  }
  if (path.startsWith("/api/v1/co-signer/intents/") && path.endsWith("/claim")) {
    return true;
  }
  return path.startsWith("/api/v1/co-signer/sessions/") && path.endsWith("/messages");
}

const explicitEmptyFormBody = requiresExplicitEmptyFormBody(method, target);
if (explicitEmptyFormBody && bodyFile) {
  fail("/api/v1/mpc/initialize requires an explicit zero-length form body; omit --body-file");
}
const body = bodyFile ? readFileSync(bodyFile) : Buffer.alloc(0);
const includeBodyHash = Boolean(bodyFile) || requiresBodyHash(method, target);
const bodyHash = includeBodyHash ? createHash("sha256").update(body).digest("hex") : "";
const canonical = [method, target, bodyHash, timestamp, nonce, keyId].join("\n");
const privateKey = privateKeyFile ? readFileSync(privateKeyFile, "utf8") : privateKeyInline;
const signature = sign(null, Buffer.from(canonical, "utf8"), privateKey).toString("base64");

const headers = {
  "X-Api-Key-Id": keyId,
  "X-Api-Timestamp": timestamp,
  "X-Api-Nonce": nonce,
  "X-Api-Signature": signature,
};

if (includeBodyHash) headers["X-Api-Body-Hash"] = bodyHash;
if (explicitEmptyFormBody) {
  headers["Content-Type"] = "application/x-www-form-urlencoded";
}
if (explicitIdempotencyKey || requiresIdempotency(method, target)) {
  headers["X-Idempotency-Key"] = explicitIdempotencyKey ?? `req-${nonce}`;
}
process.stdout.write(`${JSON.stringify({ canonicalRequestTarget: target, headers }, null, 2)}\n`);

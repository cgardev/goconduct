/**
 * Development mode: the Go server analyzing this repository, plus the Angular
 * development server with live reload in front of it.
 *
 *   pnpm dev:app-goconduct
 *
 * The Go server runs from source on GOCONDUCT_ADDRESS (127.0.0.1:6062 when
 * unset). The Angular server runs on port 4200 and proxies every Connect call
 * to that address, so the console at http://127.0.0.1:4200 shows the live
 * analysis while every source edit reloads the page.
 *
 * Both processes stop together: when one exits or the script receives an
 * interrupt, the other is terminated.
 */
import { spawn } from 'node:child_process';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const repositoryRoot = fileURLToPath(new URL('../../', import.meta.url));
const webRoot = fileURLToPath(new URL('../', import.meta.url));
const apiAddress = process.env['GOCONDUCT_ADDRESS'] ?? '127.0.0.1:6062';

/** Starts one child process and forwards its output under a name prefix. */
function start(name, command, commandArguments, cwd, environment) {
  // Each child leads its own process group, so stopping it reaches every
  // process it spawned in turn (the go run wrapper and its binary, the pnpm
  // wrapper and the Angular server).
  const child = spawn(command, commandArguments, {
    cwd,
    env: { ...process.env, ...environment },
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: true,
  });
  const forward = (stream, sink) => {
    let buffered = '';
    stream.on('data', (chunk) => {
      buffered += chunk.toString();
      const lines = buffered.split('\n');
      buffered = lines.pop() ?? '';
      for (const line of lines) {
        sink.write(`[${name}] ${line}\n`);
      }
    });
  };
  forward(child.stdout, process.stdout);
  forward(child.stderr, process.stderr);
  return child;
}

const api = start(
  'api',
  'go',
  ['run', './cmd/goconduct', '--root', '.', '--address', apiAddress],
  repositoryRoot,
  {},
);
const web = start(
  'web',
  'pnpm',
  ['exec', 'ng', 'serve', 'app-goconduct'],
  webRoot,
  { GOCONDUCT_API: `http://${apiAddress}` },
);

let exiting = false;

function stop(code) {
  if (exiting) {
    return;
  }
  exiting = true;
  for (const child of [api, web]) {
    if (child.exitCode === null && child.pid !== undefined) {
      try {
        process.kill(-child.pid, 'SIGTERM');
      } catch {
        child.kill('SIGTERM');
      }
    }
  }
  // The stdio pipes of a surviving grandchild would keep this process alive,
  // so the exit is forced after a short grace period.
  setTimeout(() => process.exit(code), 2000).unref();
  process.exitCode = code;
}

api.on('exit', (code) => {
  process.stderr.write(`[dev] the Go server exited with code ${code ?? 0}\n`);
  stop(code ?? 0);
});
web.on('exit', (code) => {
  process.stderr.write(`[dev] the Angular server exited with code ${code ?? 0}\n`);
  stop(code ?? 0);
});
process.on('SIGINT', () => stop(0));
process.on('SIGTERM', () => stop(0));

process.stderr.write(
  `[dev] Go server on http://${apiAddress} · console on http://127.0.0.1:4200\n`,
);

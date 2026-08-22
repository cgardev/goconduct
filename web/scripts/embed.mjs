import { copyFile, cp, mkdir, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const source = fileURLToPath(new URL('../dist/app-goconduct/browser/', import.meta.url));
const thirdPartyLicenses = fileURLToPath(
  new URL('../dist/app-goconduct/3rdpartylicenses.txt', import.meta.url),
);
const target = fileURLToPath(
  new URL('../../plugin/architecture/_resources/web/', import.meta.url),
);

await rm(target, { recursive: true, force: true });
await mkdir(target, { recursive: true });
await cp(source, target, { recursive: true });
await copyFile(thirdPartyLicenses, join(target, '3rdpartylicenses.txt'));

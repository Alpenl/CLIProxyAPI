import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

test('management UI uses /data import path by default', () => {
  assert.match(html, /const DEFAULT_IMPORT_PATH = "\/data\/import";/);
});

test('management UI draft config uses /data auth directory by default', () => {
  assert.match(html, /authDir: "\/data\/auths",/);
});

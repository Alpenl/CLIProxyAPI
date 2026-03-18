import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

test('management UI pauses quota auto refresh when document is hidden', () => {
  assert.match(html, /document\.addEventListener\("visibilitychange"/);
  assert.match(html, /document\.hidden/);
  assert.match(html, /refreshQuotaCollection\(\{ onlyStale: true, silentLog: true \}\)/);
});

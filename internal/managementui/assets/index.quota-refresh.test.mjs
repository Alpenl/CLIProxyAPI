import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

async function loadRefreshQuota() {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const startSignature = 'async function refreshQuota(name, options = {}) {';
  const endSignature = '\n\n      async function refreshQuotaCollection(options = {}) {';
  const start = html.indexOf(startSignature);
  const end = html.indexOf(endSignature, start);
  assert.notEqual(start, -1, 'refreshQuota function not found');
  assert.notEqual(end, -1, 'refreshQuota function end not found');
  return html.slice(start, end).trim();
}

function createContext({ previousQuota }) {
  const logs = [];
  const now = 1_763_646_500_000;
  const state = {
    accounts: [
      {
        name: 'demo.json',
        id_token: { plan_type: 'plus' },
      },
    ],
    quotaByAccount: previousQuota ? { 'demo.json': previousQuota } : {},
  };

  const context = vm.createContext({
    state,
    logs,
    ensureManagementKey: () => true,
    fetchCodexQuota: async () => {
      throw new Error('refresh failed');
    },
    normalizePlanType: (value) => value ?? null,
    formatQuotaErrorCopy: (message) => ({ summary: `friendly:${message}`, detail: message }),
    renderApp: () => {},
    addLog: (message, level) => logs.push({ message, level }),
    Date: { now: () => now },
  });

  return { context, logs, now, state };
}

test('refreshQuota records failure time when no cached quota exists', async () => {
  const functionSource = await loadRefreshQuota();
  const { context, logs, now, state } = createContext({ previousQuota: null });

  vm.runInContext(`${functionSource}\nthis.refreshQuota = refreshQuota;`, context);

  const ok = await context.refreshQuota('demo.json');

  assert.equal(ok, false);
  assert.equal(state.quotaByAccount['demo.json'].status, 'error');
  assert.equal(state.quotaByAccount['demo.json'].fetchedAt, now);
  assert.deepEqual(logs, [{ message: '刷新 demo.json 的额度失败：friendly:refresh failed', level: 'error' }]);
});

test('refreshQuota refreshes failure timestamp even when falling back to cached windows', async () => {
  const functionSource = await loadRefreshQuota();
  const previousQuota = {
    status: 'success',
    windows: [{ id: 'weekly', label: '7 天', usedPercent: 20, resetLabel: '03/20 12:00' }],
    planType: 'plus',
    fetchedAt: 10,
    refreshing: false,
    lastError: '',
  };
  const { context, logs, now, state } = createContext({ previousQuota });

  vm.runInContext(`${functionSource}\nthis.refreshQuota = refreshQuota;`, context);

  const ok = await context.refreshQuota('demo.json');

  assert.equal(ok, false);
  assert.equal(state.quotaByAccount['demo.json'].status, 'success');
  assert.equal(state.quotaByAccount['demo.json'].fetchedAt, now);
  assert.equal(state.quotaByAccount['demo.json'].lastError, 'friendly:refresh failed');
  assert.deepEqual(logs, [{ message: '刷新 demo.json 的额度失败：friendly:refresh failed', level: 'error' }]);
});

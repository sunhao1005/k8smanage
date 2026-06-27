// ai-kit 跨工具 hook 公共库（Claude Code / Codex 通用）
// 三档动作(warn/ask/deny) + 严重度策略 + 审计日志。
// 已核实：Claude PreToolUse 支持 allow/deny/ask/defer；Codex 仅 allow/deny（ask 不支持）。
// 故 ask 仅在 interactive=true（用户在 Claude 下显式开）时才发出，否则退回 deny/warn——绝不向 Codex 发 ask。
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const HERE = path.dirname(fileURLToPath(import.meta.url)); // .agents/hooks
const POLICY_FILE = path.join(HERE, 'policy.json');
const AUDIT_FILE = path.join(path.dirname(HERE), '.audit.jsonl'); // .agents/.audit.jsonl

export async function readStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  return Buffer.concat(chunks).toString('utf8');
}

export function extractCommand(raw) {
  try {
    const j = JSON.parse(raw);
    return j?.tool_input?.command || j?.toolInput?.command || j?.tool_input?.cmd || j?.command || '';
  } catch {
    return raw || '';
  }
}

// 读 stdin 并 JSON 解析（PostToolUse 等需要整个输入对象）
export async function readInput() {
  try { return JSON.parse(await readStdin()); } catch { return {}; }
}

const DEFAULT_POLICY = {
  enforceLevel: 'grey',
  interactive: false,
  map: {
    grey: { critical: 'warn', high: 'warn', medium: 'warn' },
    normal: { critical: 'deny', high: 'deny', medium: 'warn' },
    strict: { critical: 'deny', high: 'deny', medium: 'deny' },
  },
  interactiveMap: {
    normal: { critical: 'deny', high: 'ask', medium: 'warn' },
    strict: { critical: 'deny', high: 'deny', medium: 'ask' },
  },
};

function loadPolicy() {
  let p = DEFAULT_POLICY;
  try { p = { ...DEFAULT_POLICY, ...JSON.parse(fs.readFileSync(POLICY_FILE, 'utf8')) }; } catch {}
  const env = process.env.AI_KIT_HOOK_ENFORCE;
  if (env === '1') p.enforceLevel = 'normal';               // 兼容旧值
  else if (['grey', 'normal', 'strict'].includes(env)) p.enforceLevel = env;
  // env 为 '0'/空/未设 → 保留文件/默认
  if (process.env.AI_KIT_INTERACTIVE === '1') p.interactive = true;
  return p;
}

function decideAction(severity, policy) {
  const lvl = policy.enforceLevel || 'grey';
  let action;
  if (policy.interactive && policy.interactiveMap?.[lvl]) action = policy.interactiveMap[lvl][severity];
  if (!action) action = (policy.map[lvl] || policy.map.grey)[severity] || 'warn';
  // 安全兜底：ask 但未开 interactive → 退回 map（绝不静默放行）
  if (action === 'ask' && !policy.interactive) action = (policy.map[lvl] || policy.map.grey)[severity] || 'warn';
  return action;
}

function detectTool() {
  const e = process.env;
  if (e.CLAUDECODE || e.CLAUDE_CODE || e.CLAUDE_PROJECT_DIR) return 'claude';
  if (e.CODEX_HOME || e.CODEX_SANDBOX || e.CODEX) return 'codex';
  return 'unknown';
}

function audit(entry) {
  try {
    fs.appendFileSync(AUDIT_FILE, JSON.stringify({ ts: new Date().toISOString(), tool: detectTool(), event: 'PreToolUse', ...entry }) + '\n');
  } catch {}
}

// 命中处理：handleHit({ hook, rule, severity, reason, cmd, redact })
export function handleHit({ hook, rule, severity = 'high', reason, cmd = '', redact = false }) {
  const policy = loadPolicy();
  const action = decideAction(severity, policy);
  audit({ hook, rule, severity, action, cmd: redact ? '<redacted>' : cmd.slice(0, 200) });

  if (action === 'deny' || action === 'ask') {
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: action, permissionDecisionReason: `[ai-kit] ${reason}` },
    }));
    process.exit(0);
  }
  // warn
  process.stderr.write(
    `\n[ai-kit ⚠️ ${severity}] ${reason}\n` +
    (policy.enforceLevel === 'grey' ? '（灰度模式：仅警告。设 AI_KIT_HOOK_ENFORCE=normal 起强制拦截）\n' : '')
  );
  process.exit(0);
}

// 未命中：放行（exit 0 + 无 JSON = defer 到正常权限流；绝不输出 allow）
export function allow() { process.exit(0); }

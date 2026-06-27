#!/usr/bin/env node
// 凭据守卫：拦凭据"值"出现 + 凭据文件加入 git。
// 动作由 policy.json + 环境变量决定（默认 grey 只警告）。
import { readStdin, extractCommand, handleHit, allow } from './_lib.mjs';

const cmd = extractCommand(await readStdin());
if (!cmd) allow();

const hit = (severity, reason) =>
  handleHit({ hook: 'guard-credentials', rule: reason, severity, reason: `${reason}——凭据绝不入库/不外泄：放本地 + .gitignore，会提交的内容只引用「凭据位置」。`, cmd, redact: true });

// 1) 凭据「值」模式：任何命令里出现都高危（不只 git）
const VALUE_PATTERNS = [
  [/-----BEGIN\s+([A-Z]+\s+)?PRIVATE KEY-----/i, 'critical', '命令含私钥内容'],
  [/\bAKIA[0-9A-Z]{16}\b/, 'critical', '命令含 AWS Access Key（AKIA…）'],
  [/\bgh[posru]_[A-Za-z0-9]{30,}\b/, 'critical', '命令含 GitHub token（ghp_/gho_…）'],
  [/\bsk-[A-Za-z0-9]{20,}\b/, 'high', '命令含疑似 API key（sk-…）'],
  [/\b(postgres|postgresql|mysql|mongodb(\+srv)?):\/\/[^\s:@/]+:[^\s:@/]+@/i, 'high', '命令含带密码的数据库连接串'],
];
for (const [re, severity, reason] of VALUE_PATTERNS) {
  if (re.test(cmd)) hit(severity, reason);
}

// 2) git add / commit -a 把凭据文件加入版本库
const isGitStage = /\bgit\s+add\b/i.test(cmd) || /\bgit\s+commit\b[^\n]*-a\b/i.test(cmd);
if (isGitStage) {
  const SECRET_FILE = /(^|[\s"'/\\])(\.env(\.[\w-]+)?|id_rsa|id_ed25519|.*\.pem|.*\.key|.*\.p12|.*\.pfx|.*secret.*|.*password.*|.*credential.*|.*\bpwd\b.*)([\s"']|$)/i;
  const EXAMPLE = /\.(env\.)?(example|sample|template|dist)\b/i;
  for (const t of cmd.split(/\s+/)) {
    if (SECRET_FILE.test(t) && !EXAMPLE.test(t)) hit('critical', `疑似把凭据/密钥文件加入 git：「${t}」`);
  }
  // 笼统 git add . / -A / * ：始终只提醒（不随强制档拦正常流程）
  if (/\bgit\s+add\s+(-A\b|--all\b|\.(?=\s|$)|\*(?=\s|$))/i.test(cmd)) {
    process.stderr.write('\n[ai-kit ⚠️] git add 全量提交——先确认没把 .env/密钥/dump 裹进去（应在 .gitignore 内）。\n');
  }
}

allow();

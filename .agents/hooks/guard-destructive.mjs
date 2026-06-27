#!/usr/bin/env node
// 破坏性命令守卫：按严重度(critical/high/medium)分档拦截。
// 动作由 policy.json + 环境变量决定（默认 grey 只警告）。
import { readStdin, extractCommand, handleHit, allow } from './_lib.mjs';

// 强推匹配：--force(非 --force-with-lease) 或 短 flag -f
const FORCE = String.raw`(--force(?!-with-lease)|\s-f(\s|$))`;

// [正则, 严重度, 原因]；critical 在前，命中即处理（handleHit 会退出）。
const RULES = [
  // ---- critical ----
  [/\brm\s+-[a-z]*r[a-z]*f[a-z]*\s+(-[a-z]+\s+)*(\/|~|\$HOME|\/\*|\*|\.)(\s|$)/i, 'critical', 'rm -rf 指向根/家目录/通配——毁灭性删除'],
  [/\b(mkfs\b|dd\s+if=.*of=\/dev\/)/i, 'critical', '磁盘级操作（mkfs / dd 写设备）'],
  [/\bdrop\s+database\b/i, 'critical', 'DROP DATABASE 删整库'],
  [/>\s*\/etc\//i, 'critical', '覆写 /etc 系统配置'],
  [/>\s*\/dev\/sd[a-z]/i, 'critical', '直接写裸盘设备'],
  [/\b(curl|wget)\b[^|]*\|\s*(sudo\s+)?(ba)?sh\b/i, 'critical', 'curl|bash 远程脚本直接执行'],
  [new RegExp(String.raw`\bgit\s+push\b[^\n]*${FORCE}[^\n]*\b(main|master|prod|production|release)\b`, 'i'), 'critical', 'git push -f/--force 到保护分支会覆盖远端历史'],
  // ---- high ----
  [/\brm\s+-[a-z]*r[a-z]*f|\brm\s+-[a-z]*f[a-z]*r|\brm\s+-r\s+-f|\brm\s+-f\s+-r/i, 'high', 'rm -rf 递归强删'],
  [/\bgit\s+reset\s+--hard\b/i, 'high', 'git reset --hard 丢弃工作区改动'],
  [/\bgit\s+clean\s+-[a-z]*f/i, 'high', 'git clean -f 删除未跟踪文件'],
  [new RegExp(String.raw`\bgit\s+push\b[^\n]*${FORCE}`, 'i'), 'high', 'git push -f/--force 可能覆盖远端历史'],
  [/\b(drop|truncate)\s+table\b/i, 'high', 'DROP/TRUNCATE TABLE 删表/清空'],
  [/\bkubectl\s+delete\b/i, 'high', 'kubectl delete 可能误删一片资源（先确认 ns + selector）'],
  [/\bdocker\s+(volume\s+rm|system\s+prune\s+-a|rm\s+-f)\b/i, 'high', 'docker 删卷/清理可能丢数据'],
  [/\bchmod\s+-?R?\s*777\b/i, 'high', 'chmod 777 放开全部权限'],
  // ---- medium ----
  [/\bgit\s+push\b[^\n]*--force-with-lease/i, 'medium', 'git push --force-with-lease 强推（较安全但仍覆盖远端）'],
  [/\bgit\s+(checkout|restore)\s+(--\s+)?\.(\s|$)/i, 'medium', 'git checkout/restore . 丢弃工作区改动'],
];

const cmd = extractCommand(await readStdin());
if (!cmd) allow();

// 把 commit/tag 等的 -m/--message 信息段剔除再匹配，避免"信息里含危险词"的误报
// （psql/bash 用 -c/--command 不受影响，仍会被扫描）
const scan = cmd.replace(/(^|\s)(-m|--message)(=|\s+)("[^"]*"|'[^']*'|\S+)/gi, ' ');

const hit = (severity, reason) =>
  handleHit({ hook: 'guard-destructive', rule: reason, severity, reason: `破坏性命令：${reason}。先确认目标、备份/快照、能 dry-run 就 dry-run。`, cmd });

for (const [re, severity, reason] of RULES) {
  if (re.test(scan)) hit(severity, reason);
}

// SQL DELETE/UPDATE 疑似缺 WHERE（全表操作）——单独判，降误报
if (/\bdelete\s+from\s+[\w."'`]+/i.test(scan) && !/\bwhere\b/i.test(scan)) hit('high', 'SQL DELETE FROM 疑似缺 WHERE（全表删除）');
if (/\bupdate\s+[\w."'`]+\s+set\b/i.test(scan) && !/\bwhere\b/i.test(scan)) hit('high', 'SQL UPDATE 疑似缺 WHERE（全表更新）');

allow();

#!/usr/bin/env node
// SessionStart 维护提醒：隔 N 天（默认 14，可用 AI_KIT_MAINT_DAYS 改）在会话开始时提醒一次
// 是否整理 ai-kit。只提醒、不清理、不阻塞、不碰项目代码。
import fs from 'node:fs';
import path from 'node:path';

const DAYS = Number(process.env.AI_KIT_MAINT_DAYS || 14);
const interval = DAYS * 24 * 60 * 60 * 1000;
const stampFile = path.resolve('.agents/.last-maintenance-reminder');
const now = Date.now();

let last = 0;
try { last = Number(fs.readFileSync(stampFile, 'utf8').trim()) || 0; } catch {}

if (now - last >= interval) {
  try { fs.writeFileSync(stampFile, String(now)); } catch {}
  if (last !== 0) { // 首次只起步计时、不打扰
    process.stdout.write(
      `\n[ai-kit 维护提醒] 距上次提醒已满 ${DAYS} 天。\n` +
      `如需整理 ai-kit（清理重复/失效/被取代的规则与技能），跟我说一句「按 self-evolve 整理」即可。\n` +
      `我会先列候选 + 逐条说理由给你确认，只动 ai-kit 自身的规则/技能文件，` +
      `绝不碰项目代码、不影响当前正在做的功能与需求；与当前任务相关或正在用的一律不动。\n`
    );
  }
}
process.exit(0);

#!/usr/bin/env node
// ai-kit 一致性校验：技能索引↔技能目录、hook 配置↔脚本、SKILL.md frontmatter。
// 默认只报告并 exit 0；加 --strict 时若有不一致 exit 1（供 CI）。
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));      // .agents/hooks
const AGENTS_DIR = path.dirname(HERE);                          // .agents
const KIT_ROOT = path.dirname(AGENTS_DIR);                      // kit 根
const SKILLS_DIR = path.join(AGENTS_DIR, 'skills');

const problems = [];
const ok = [];
const read = (p) => { try { return fs.readFileSync(p, 'utf8'); } catch { return null; } };

// 1) 技能目录 + frontmatter
let skillNames = [];
try {
  skillNames = fs.readdirSync(SKILLS_DIR, { withFileTypes: true }).filter((d) => d.isDirectory()).map((d) => d.name);
} catch { problems.push(`找不到技能目录 ${SKILLS_DIR}`); }

for (const name of skillNames) {
  const sk = read(path.join(SKILLS_DIR, name, 'SKILL.md'));
  if (!sk) { problems.push(`技能 ${name}: 缺 SKILL.md`); continue; }
  const fm = sk.match(/^---\s*\n([\s\S]*?)\n---/);
  if (!fm) { problems.push(`技能 ${name}: 缺 frontmatter`); continue; }
  const fmName = (fm[1].match(/^name:\s*(.+)$/m) || [])[1]?.trim();
  const fmDesc = (fm[1].match(/^description:\s*(.+)$/m) || [])[1]?.trim();
  if (!fmName) problems.push(`技能 ${name}: frontmatter 缺 name`);
  else if (fmName !== name) problems.push(`技能 ${name}: name(${fmName}) 与目录名不一致`);
  if (!fmDesc) problems.push(`技能 ${name}: frontmatter 缺 description`);
  if (fmName === name && fmDesc) ok.push(`技能 ${name} ✓`);
}

// 2) 技能 ↔ AGENTS.md 索引 双向
const agents = read(path.join(KIT_ROOT, 'AGENTS.md')) || '';
const idxSection = (agents.split(/##\s*技能索引/)[1] || '').split(/\n##\s/)[0];
for (const name of skillNames) {
  if (!agents.includes(name)) problems.push(`技能 ${name}: 未出现在 AGENTS.md`);
}
for (const m of idxSection.matchAll(/\*\*([a-z][a-z0-9-]+)\*\*/g)) {
  if (!skillNames.includes(m[1])) problems.push(`索引引用了不存在的技能：${m[1]}`);
}

// 3) hook 配置 ↔ 脚本存在
for (const cfgPath of [path.join(KIT_ROOT, '.claude/settings.json'), path.join(KIT_ROOT, '.codex/hooks.json')]) {
  const raw = read(cfgPath);
  if (!raw) { problems.push(`缺 hook 配置 ${path.relative(KIT_ROOT, cfgPath)}`); continue; }
  for (const m of raw.matchAll(/\.agents\/hooks\/([\w.-]+\.mjs)/g)) {
    if (!fs.existsSync(path.join(AGENTS_DIR, 'hooks', m[1]))) problems.push(`${path.relative(KIT_ROOT, cfgPath)} 引用了不存在的脚本：${m[1]}`);
  }
}

// 4) policy.json 合法
const pol = read(path.join(HERE, 'policy.json'));
if (pol) {
  try {
    const j = JSON.parse(pol);
    if (!['grey', 'normal', 'strict'].includes(j.enforceLevel)) problems.push(`policy.json: enforceLevel 取值非法（${j.enforceLevel}）`);
  } catch (e) { problems.push(`policy.json: JSON 解析失败 ${e.message}`); }
}

// 输出
console.log(`[ai-kit verify] 技能 ${skillNames.length} 个，通过项 ${ok.length}`);
if (problems.length === 0) {
  console.log('✅ 全部一致，无问题。');
  process.exit(0);
}
console.log(`⚠️ 发现 ${problems.length} 处不一致：`);
for (const p of problems) console.log('  - ' + p);
process.exit(process.argv.includes('--strict') ? 1 : 0);

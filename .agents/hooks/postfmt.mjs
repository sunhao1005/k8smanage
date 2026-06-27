#!/usr/bin/env node
// PostToolUse(Edit|Write)：可选自动格式化刚改的文件。
// 默认关（会改文件+依赖项目装了 formatter）；设 AI_KIT_AUTOFORMAT=1 开启。
import { readInput } from './_lib.mjs';
import { execSync } from 'node:child_process';
import fs from 'node:fs';

if (process.env.AI_KIT_AUTOFORMAT !== '1') process.exit(0);

const input = await readInput();
const fp = input?.tool_input?.file_path || input?.tool_input?.path;
if (!fp || !fs.existsSync(fp)) process.exit(0);

const ext = fp.split('.').pop().toLowerCase();
const tryRun = (cmd) => { try { execSync(cmd, { stdio: 'ignore' }); return true; } catch { return false; } };

let done = false;
if (['js', 'jsx', 'ts', 'tsx', 'json', 'css', 'scss', 'less', 'md', 'mjs', 'cjs', 'yaml', 'yml', 'html', 'vue'].includes(ext)) {
  done = tryRun(`npx --no-install prettier --write "${fp}"`); // 仅当项目本地装了 prettier 才生效
} else if (ext === 'go') {
  done = tryRun(`gofmt -w "${fp}"`);
} else if (ext === 'py') {
  done = tryRun(`ruff format "${fp}"`) || tryRun(`black "${fp}"`);
} else if (ext === 'rs') {
  done = tryRun(`rustfmt "${fp}"`);
}
if (done) process.stderr.write(`[ai-kit] 已自动格式化 ${fp}\n`);
process.exit(0);

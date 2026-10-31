#!/usr/bin/env node
// Static security scan of hub skills. Reads the rules in rules/rules.txt (plus
// the hub's config/audit-rules.txt), scans every file of a skill and prints the
// findings as evidence for a reviewer. It never executes anything it scans and
// makes no network calls. No findings does not mean a skill is safe.
//
//   node audit.mjs <id|folder>... | --all  [--json] [--fail-on high] [--hub DIR] [--rules FILE]
//   node audit.mjs --all --changed       only skills changed since their recorded audit
//   node audit.mjs --record <id> --verdict ok|review|block   record a finished audit
//   node audit.mjs --self-test            run every rule's match/clean examples
//
// The hub owns skill metadata, so hub skills (an id or --all) are located and
// their recorded audits read through `skillstation skill audit-status`, and a
// record is written through `skillstation skill audit-record`. The binary is
// SKILLSTATION_BIN or skillstation on PATH. A folder named by its path is
// scanned without the binary and has no recorded audit.
//
// Exit codes: 0 nothing at or above --fail-on, 1 findings at or above it, 2 usage or read error.
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SEVERITIES = ['info', 'low', 'medium', 'high', 'critical'];
const MAX_TEXT_BYTES = 2 * 1024 * 1024;
const MAX_TOTAL_BYTES = 8 * 1024 * 1024;
const MAX_SKILL_MD_LINES = 500;
const SKIPPED_DIRS = new Set(['.git', '.meta']);
const MEDIA_EXTENSIONS = new Set(['.png', '.jpg', '.jpeg', '.gif', '.webp', '.ico', '.ttf', '.otf', '.woff', '.woff2', '.mp4', '.mov', '.webm', '.mp3', '.wav', '.pdf']);
const SCRIPT_EXTENSIONS = new Set(['.sh', '.bash', '.py', '.mjs', '.js', '.cjs', '.ts', '.rb', '.pl', '.ps1']);
const here = path.dirname(fileURLToPath(import.meta.url));

// parseRules reads the plain-text rule format documented in SKILL.md.
export function parseRules(text, origin) {
  const rules = [];
  let rule = null;
  const fail = (lineNo, message) => { throw new Error(`${origin}:${lineNo}: ${message}`); };
  text.split(/\r?\n/).forEach((raw, index) => {
    const lineNo = index + 1;
    if (raw.startsWith('## ')) {
      rule = { id: raw.slice(3).trim(), patterns: [], match: [], clean: [], origin, line: lineNo };
      rules.push(rule);
      return;
    }
    if (raw.trim() === '' || raw.startsWith('#')) return;
    const colon = raw.indexOf(':');
    if (colon < 1 || !rule) fail(lineNo, `expected "key: value" inside a rule, got "${raw.slice(0, 40)}"`);
    const key = raw.slice(0, colon).trim();
    const value = raw.slice(colon + 1).trim();
    if (key === 'pattern') rule.patterns.push(value);
    else if (key === 'match') rule.match.push(value);
    else if (key === 'clean') rule.clean.push(value);
    else if (['category', 'severity', 'files', 'scope', 'flags', 'check', 'why', 'fix', 'source', 'redact'].includes(key)) rule[key] = value;
    else fail(lineNo, `unknown key "${key}"`);
  });
  for (const r of rules) validateRule(r);
  return rules;
}

function validateRule(rule) {
  const where = `${rule.origin}:${rule.line} (${rule.id})`;
  if (!/^[a-z0-9]+(\.[a-z0-9-]+)+$/.test(rule.id)) throw new Error(`${where}: id must look like category.name`);
  if (!SEVERITIES.includes(rule.severity)) throw new Error(`${where}: severity must be one of ${SEVERITIES.join(', ')}`);
  if (!rule.why) throw new Error(`${where}: a rule needs a "why" line`);
  if (!rule.check && rule.patterns.length === 0) throw new Error(`${where}: a rule needs a pattern or a check`);
  rule.category = rule.category || rule.id.split('.')[0];
  rule.fileGlobs = (rule.files || '*').split(',').map((g) => g.trim()).filter(Boolean);
  rule.regexes = rule.patterns.map((p) => {
    try { return new RegExp(p, rule.flags ?? 'iu'); } catch (error) { throw new Error(`${where}: bad pattern: ${error.message}`); }
  });
}

function globToRegExp(glob) {
  let source = '';
  for (let i = 0; i < glob.length; i++) {
    const c = glob[i];
    if (c === '*' && glob[i + 1] === '*') { source += '.*'; i++; if (glob[i + 1] === '/') i++; }
    else if (c === '*') source += '[^/]*';
    else source += c.replace(/[.+^${}()|[\]\\?]/g, '\\$&');
  }
  return new RegExp(`^${source}$`);
}

function fileMatches(rule, relPath) {
  return rule.fileGlobs.some((glob) => globToRegExp(glob).test(glob.includes('/') ? relPath : path.posix.basename(relPath)));
}

const sha256 = (data) => createHash('sha256').update(data).digest('hex');
const lineOf = (text, index) => text.slice(0, index).split('\n').length;
const fingerprint = (rule, file, evidence) => createHash('sha256').update(`${rule}\n${file}\n${evidence.replace(/\s+/g, ' ').trim()}`).digest('hex').slice(0, 16);

// matchText applies the regex rules to one text. A rule with several patterns
// needs all of them somewhere in the file; the first one locates the evidence.
function matchText(rule, text) {
  if (rule.regexes.length === 0) return [];
  const lines = text.split('\n');
  const hits = [];
  if (rule.regexes.length > 1 || rule.scope === 'file') {
    const located = rule.regexes.map((re) => { const flags = re.flags.replace('g', ''); return new RegExp(re.source, flags).exec(text); });
    if (located.some((m) => !m)) return [];
    const line = lineOf(text, located[0].index);
    hits.push({ line, evidence: located[0][0].split('\n')[0] });
    return hits;
  }
  lines.forEach((content, index) => {
    const m = rule.regexes[0].exec(content);
    if (m) hits.push({ line: index + 1, evidence: content });
  });
  return hits;
}

function readFrontmatter(text) {
  const m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text);
  return m ? m[1] : null;
}

function frontmatterValue(front, key) {
  const m = new RegExp(`^${key}:[ \\t]*(.*)$`, 'm').exec(front);
  if (!m) return null;
  let value = m[1].trim();
  if (value === '' || value === '>-' || value === '>' || value === '|') {
    const rest = front.slice(m.index + m[0].length).split('\n').slice(1);
    const block = [];
    for (const line of rest) { if (/^\s+\S/.test(line) || /^\s*-\s/.test(line)) block.push(line.trim()); else break; }
    value = block.join(' ');
  }
  return value;
}

// builtinChecks hold the rules that look at file facts instead of text.
const builtinChecks = {
  symlink: (file) => {
    if (!file.symlink) return [];
    const outside = file.symlinkOutside;
    return [{ line: 0, evidence: `symbolic link to ${file.symlinkTarget}`, severity: outside ? 'critical' : 'high' }];
  },
  // An image, font or media file with no executable magic is data: shown, never failing.
  binary: (file) => {
    if (!file.binary) return [];
    const data = file.binary === 'binary' && MEDIA_EXTENSIONS.has(path.extname(file.rel).toLowerCase());
    return [{ line: 0, evidence: `${data ? 'media or font asset' : `${file.binary} file`}, ${file.size} bytes`, severity: data ? 'info' : undefined }];
  },
  size: (file) => (file.size > MAX_TEXT_BYTES ? [{ line: 0, evidence: `${file.size} bytes, over ${MAX_TEXT_BYTES}` }]
    : file.rel === 'SKILL.md' && file.text && file.text.split('\n').length > MAX_SKILL_MD_LINES ? [{ line: 0, evidence: `SKILL.md has more than ${MAX_SKILL_MD_LINES} lines` }] : []),
  'exec-bit': (file) => (file.executable && !SCRIPT_EXTENSIONS.has(path.extname(file.rel).toLowerCase()) ? [{ line: 0, evidence: 'executable bit on a file that is not a script' }] : []),
  'allowed-tools': (file) => {
    if (file.rel !== 'SKILL.md' || !file.text) return [];
    const front = readFrontmatter(file.text);
    const value = front && frontmatterValue(front, 'allowed-tools');
    if (!value) return [];
    const broad = /(^|[\s,\[])Bash(\(\*\)|\(curl[^)]*\)|\(wget[^)]*\))?(?=[\s,\]]|$)/i.exec(value);
    return broad ? [{ line: 0, evidence: `allowed-tools: ${value}`.slice(0, 160) }] : [];
  },
  lifecycle: (file) => {
    if (path.posix.basename(file.rel) !== 'package.json' || !file.text) return [];
    try {
      const scripts = JSON.parse(file.text).scripts ?? {};
      return ['preinstall', 'install', 'postinstall', 'prepare'].filter((k) => k in scripts).map((k) => ({ line: 0, evidence: `scripts.${k}: ${String(scripts[k]).slice(0, 120)}` }));
    } catch { return []; }
  },
  meta: (file) => {
    if (file.rel !== 'SKILL.md' || !file.text) return [];
    const front = readFrontmatter(file.text);
    if (front === null) return [{ line: 1, evidence: 'SKILL.md has no frontmatter' }];
    const found = [];
    const name = frontmatterValue(front, 'name') ?? '';
    const description = frontmatterValue(front, 'description') ?? '';
    if (!name || !description) found.push({ line: 1, evidence: 'frontmatter lacks a name or a description' });
    if (/anthropic|claude/i.test(name)) found.push({ line: 1, evidence: `name "${name}" uses a reserved word` });
    if (description.length > 1024) found.push({ line: 1, evidence: `description is ${description.length} characters` });
    if (/<[a-z][^>]*>/i.test(description)) found.push({ line: 1, evidence: 'description holds markup' });
    return found;
  },
};

function binaryKind(buffer) {
  const head = buffer.subarray(0, 4);
  const hex = head.toString('hex');
  if (hex === '7f454c46') return 'ELF';
  if (['feedface', 'feedfacf', 'cafebabe', 'cffaedfe', 'cefaedfe'].includes(hex)) return 'Mach-O';
  if (hex.startsWith('4d5a')) return 'PE';
  if (hex === '504b0304') return 'zip archive';
  return buffer.subarray(0, 8000).includes(0) ? 'binary' : '';
}

// collectFiles lists every file of a skill folder with the facts rules need.
export function collectFiles(skillDir) {
  const root = fs.realpathSync(skillDir);
  const files = [];
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      const full = path.join(dir, entry.name);
      const rel = path.relative(root, full).split(path.sep).join('/');
      if (entry.isDirectory()) { if (!SKIPPED_DIRS.has(entry.name)) walk(full); continue; }
      const stat = fs.lstatSync(full);
      const file = { rel, size: stat.size, executable: (stat.mode & 0o111) !== 0, text: null };
      if (stat.isSymbolicLink()) {
        file.symlink = true;
        file.symlinkTarget = fs.readlinkSync(full);
        let resolved = '';
        try { resolved = fs.realpathSync(full); } catch { resolved = ''; }
        file.symlinkOutside = !resolved || (resolved !== root && !resolved.startsWith(root + path.sep));
      } else if (stat.isFile() && stat.size <= MAX_TEXT_BYTES) {
        const buffer = fs.readFileSync(full);
        const kind = binaryKind(buffer) || (/\.(so|dylib|dll|exe|pyc|class)$/i.test(rel) ? 'binary' : '');
        if (kind) file.binary = kind; else file.text = buffer.toString('utf8').replace(/^﻿/, '');
      } else if (stat.isFile()) {
        file.oversized = true;
      }
      files.push(file);
    }
  };
  walk(root);
  return files;
}

// auditFiles runs every rule over the files of one skill.
export function auditFiles(files, rules) {
  const findings = [];
  const add = (rule, file, hit) => {
    const evidence = (hit.evidence ?? '').trim().slice(0, 160);
    findings.push({
      rule: rule.id, category: rule.category, severity: hit.severity ?? rule.severity, file: file.rel, line: hit.line, evidence: rule.redact === 'true' ? '[redacted]' : evidence,
      why: rule.why, fix: rule.fix ?? '', fingerprint: fingerprint(rule.id, file.rel, evidence),
    });
  };
  for (const file of files) {
    for (const rule of rules) {
      if (!fileMatches(rule, file.rel)) continue;
      if (rule.check) {
        const run = builtinChecks[rule.check];
        if (!run) throw new Error(`${rule.origin}:${rule.line} (${rule.id}): unknown check "${rule.check}"`);
        run(file).forEach((hit) => add(rule, file, hit));
      }
      if (file.text != null) matchText(rule, file.text).forEach((hit) => add(rule, file, hit));
    }
  }
  const total = files.reduce((sum, f) => sum + f.size, 0);
  if (total > MAX_TOTAL_BYTES) {
    const rule = rules.find((r) => r.check === 'size');
    if (rule) findings.push({ rule: rule.id, category: rule.category, severity: rule.severity, file: '.', line: 0, evidence: `${total} bytes in all, over ${MAX_TOTAL_BYTES}`, why: rule.why, fix: rule.fix ?? '', fingerprint: fingerprint(rule.id, '.', String(total)) });
  }
  return findings.sort((a, b) => SEVERITIES.indexOf(b.severity) - SEVERITIES.indexOf(a.severity) || a.file.localeCompare(b.file) || a.line - b.line);
}

export function loadRules(options) {
  const rules = parseRules(fs.readFileSync(path.join(here, '..', 'rules', 'rules.txt'), 'utf8'), 'rules/rules.txt');
  const extra = options.rules ?? path.join(options.hub, 'config', 'audit-rules.txt');
  if (fs.existsSync(extra)) rules.push(...parseRules(fs.readFileSync(extra, 'utf8'), extra));
  rules.digest = 'sha256:' + sha256(rules.map((r) => `${r.origin}\n${r.id}\n${JSON.stringify([r.severity, r.files, r.scope, r.flags, r.check, r.redact, r.patterns])}`).join('\n'));
  const seen = new Set();
  for (const rule of rules) {
    if (seen.has(rule.id)) throw new Error(`rule id ${rule.id} is defined twice (${rule.origin}:${rule.line})`);
    seen.add(rule.id);
  }
  return rules;
}

// unescapeSample turns \n and \u{XXXX} in an example into the characters they name.
const unescapeSample = (sample) => sample.replace(/\\n/g, '\n').replace(/\\u\{([0-9a-fA-F]+)\}/g, (_, hex) => String.fromCodePoint(parseInt(hex, 16)));

// selfTest checks each rule's own examples: every match line must hit the rule
// and every clean line must not. It returns the failures.
export function selfTest(rules) {
  const failures = [];
  for (const rule of rules) {
    if (!rule.check && (rule.match.length === 0 || rule.clean.length === 0)) failures.push(`${rule.id}: a pattern rule needs at least one match and one clean example`);
    const hits = (sample) => matchText(rule, unescapeSample(sample)).length > 0;
    rule.match.forEach((sample) => { if (!hits(sample)) failures.push(`${rule.id}: should match "${sample}"`); });
    rule.clean.forEach((sample) => { if (hits(sample)) failures.push(`${rule.id}: should not match "${sample}"`); });
  }
  return failures;
}

// runBinary runs a skillstation command with --json and returns its parsed
// output. The binary owns the hub's files: it locates skills, computes their
// content digest and reads and writes the recorded audit.
function runBinary(args, what) {
  const binary = process.env.SKILLSTATION_BIN || 'skillstation';
  const run = spawnSync(binary, [...args, '--json'], { encoding: 'utf8' });
  if (run.error) {
    throw new Error(run.error.code === 'ENOENT'
      ? `cannot ${what}: the skillstation binary "${binary}" was not found; put skillstation on PATH or set SKILLSTATION_BIN to its path`
      : `cannot ${what}: running ${binary}: ${run.error.message}`);
  }
  if (run.status !== 0) throw new Error(`skillstation ${args.slice(0, 3).join(' ')} failed (exit ${run.status}):\n${(run.stderr || run.stdout).trim()}`);
  try { return JSON.parse(run.stdout); } catch { throw new Error(`skillstation ${args.slice(0, 2).join(' ')} printed no JSON:\n${run.stdout.trim()}`); }
}

// resolveTargets turns the arguments into skills to scan. A folder holding a
// SKILL.md is scanned as it is; any other argument, and --all, names hub
// skills, which the binary locates together with their recorded audit.
function resolveTargets(options, rules) {
  const folders = [];
  const ids = [];
  for (const arg of options.targets) {
    if (fs.existsSync(path.join(arg, 'SKILL.md'))) folders.push({ id: path.basename(path.resolve(arg)), dir: arg, since: 'never-audited', recorded: null, contentDigest: null });
    else ids.push(arg);
  }
  if (!options.all && ids.length === 0) return folders;
  const args = ['skill', 'audit-status', ...(options.all ? ['--all'] : ids), '--rules-digest', rules.digest, '--workspace', options.hub];
  const status = runBinary(args, 'locate the hub skills');
  const hub = status.skills.map((s) => ({ id: s.skill_id, dir: s.dir, since: s.since_last_audit, recorded: s.recorded, contentDigest: s.content_digest }));
  return [...folders, ...hub];
}

function parseArgs(argv) {
  const options = { json: false, failOn: 'high', hub: process.cwd(), all: false, selfTest: false, changed: false, targets: [] };
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '--json') options.json = true;
    else if (arg === '--all') options.all = true;
    else if (arg === '--self-test') options.selfTest = true;
    else if (arg === '--changed') options.changed = true;
    else if (arg === '--record') options.record = argv[++i];
    else if (arg === '--verdict') options.verdict = argv[++i];
    else if (arg === '--fail-on') options.failOn = argv[++i];
    else if (arg === '--hub') options.hub = argv[++i];
    else if (arg === '--rules') options.rules = argv[++i];
    else if (arg.startsWith('--')) throw new Error(`unknown option ${arg}`);
    else options.targets.push(arg);
  }
  if (!SEVERITIES.includes(options.failOn)) throw new Error(`--fail-on must be one of ${SEVERITIES.join(', ')}`);
  if (options.record && !['ok', 'review', 'block'].includes(options.verdict)) throw new Error('--record needs --verdict ok, review or block');
  if (!options.selfTest && !options.record && !options.all && options.targets.length === 0) throw new Error('name a skill id or folder, or pass --all');
  return options;
}

function render(results, unchanged = []) {
  const lines = [];
  for (const result of results) {
    const note = result.since_last_audit === 'unchanged' ? '' : `, ${result.since_last_audit.replace('-', ' ')}`;
    lines.push(`${result.id}: ${result.verdict} (${result.files.length} files, ${result.findings.length} findings${note})`);
    for (const f of result.findings) {
      lines.push(`  [${f.severity}] ${f.rule} ${f.file}${f.line ? `:${f.line}` : ''}`);
      if (f.evidence) lines.push(`      ${f.evidence}`);
      lines.push(`      why: ${f.why}`);
    }
  }
  for (const skill of unchanged) lines.push(`${skill.id}: unchanged since the audit on ${skill.audited_at}, recorded ${skill.verdict}`);
  lines.push('', 'Static rules catch known patterns only. No findings does not mean the skill is safe.');
  return lines.join('\n');
}

function main() {
  const options = parseArgs(process.argv.slice(2));
  const rules = loadRules(options);
  if (options.selfTest) {
    const failures = selfTest(rules);
    failures.forEach((f) => console.error(f));
    console.log(failures.length === 0 ? `${rules.length} rules, all examples pass` : `${failures.length} rule examples fail`);
    return failures.length === 0 ? 0 : 1;
  }
  if (options.record) return recordAudit(options, rules);
  const unchanged = [];
  const results = [];
  for (const { id, dir, since, recorded: record, contentDigest } of resolveTargets(options, rules)) {
    if (options.changed && since === 'unchanged') {
      unchanged.push({ id, verdict: record.verdict, audited_at: record.audited_at });
      continue;
    }
    const files = collectFiles(dir);
    const findings = auditFiles(files, rules);
    results.push({
      id, verdict: findings.length ? findings[0].severity : 'clean', since_last_audit: since, content_digest: contentDigest,
      previous: record ? { verdict: record.verdict, audited_at: record.audited_at } : null, files: files.map((f) => f.rel), findings,
    });
  }
  const threshold = SEVERITIES.indexOf(options.failOn);
  const failed = results.some((r) => r.findings.some((f) => SEVERITIES.indexOf(f.severity) >= threshold));
  console.log(options.json ? JSON.stringify({ rules: rules.length, fail_on: options.failOn, skills: results, unchanged }, null, 2) : render(results, unchanged));
  return failed ? 1 : 0;
}

// recordAudit stores the verdict of a finished audit with the digest of the
// rules, so a later --changed run skips the skill until it or the rules change.
// The binary computes the digest of the skill's content and writes the record
// the way every hub change is written: locked, atomic, validated, committed.
function recordAudit(options, rules) {
  const id = options.record;
  const result = runBinary(['skill', 'audit-record', id, '--verdict', options.verdict, '--rules-digest', rules.digest, '--workspace', options.hub], 'record');
  console.log(`${id}: recorded ${options.verdict} for content ${result.content_digest}`);
  return 0;
}

if (process.argv[1] && fs.realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exitCode = main(); } catch (error) { console.error(`audit: ${error.message}`); process.exitCode = 2; }
}

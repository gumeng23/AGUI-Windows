import * as cp from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as vscode from 'vscode';
import {
  ALL_ENTRIES,
  BUILTINS,
  EVENTS,
  GUI_FUNCTIONS,
  KEYWORDS,
  METHODS,
  PROPERTIES,
  WIDGETS,
  type ApiEntry,
  type ApiKind,
} from './aguiData';

const LANGUAGE_ID = 'agui';

/** 扩展安装目录，activate 时赋值。 */
let extensionRoot = '';
/** 当前用于运行 / 打包的终端，保证同一时间只有一个。 */
let activeTerminal: vscode.Terminal | undefined;
let linter: AguiLinter | undefined;

// ---------------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------------

function settings(): vscode.WorkspaceConfiguration {
  return vscode.workspace.getConfiguration('agui');
}

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function isFile(target: string): boolean {
  try {
    return fs.statSync(target).isFile();
  } catch {
    return false;
  }
}

function quoteShell(value: string): string {
  return /[\s"]/.test(value) ? `"${value.replace(/"/g, '\\"')}"` : value;
}

function workingDirectory(doc: vscode.TextDocument): string {
  const folder = vscode.workspace.getWorkspaceFolder(doc.uri);
  if (folder) return folder.uri.fsPath;
  return doc.uri.scheme === 'file' ? path.dirname(doc.uri.fsPath) : process.cwd();
}

/** 把 `agui.buildOptions` 拆成参数数组，支持单双引号。 */
function splitArgs(input: string): string[] {
  const args: string[] = [];
  const re = /"([^"]*)"|'([^']*)'|(\S+)/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(input)) !== null) {
    args.push(match[1] ?? match[2] ?? match[3] ?? '');
  }
  return args;
}

function asUri(arg: unknown): vscode.Uri | undefined {
  return arg instanceof vscode.Uri ? arg : undefined;
}

// ---------------------------------------------------------------------------
// agui.exe 定位
// ---------------------------------------------------------------------------

function executableCandidates(uri?: vscode.Uri): string[] {
  const roots = new Set<string>();
  const add = (value: string | undefined) => {
    if (value) roots.add(path.resolve(value));
  };

  const folder = uri ? vscode.workspace.getWorkspaceFolder(uri) : vscode.workspace.workspaceFolders?.[0];
  add(folder?.uri.fsPath);
  add(vscode.workspace.workspaceFolders?.[0]?.uri.fsPath);
  if (uri?.scheme === 'file') add(path.dirname(uri.fsPath));
  add(path.join(extensionRoot, '..'));
  add(path.join(extensionRoot, '..', '..'));
  add(process.env.AGUI_HOME);
  add(process.cwd());

  const names = process.platform === 'win32' ? ['agui.exe', 'agui.cmd', 'agui.bat', 'agui'] : ['agui'];
  const candidates: string[] = [];
  for (const root of roots) {
    for (const name of names) candidates.push(path.join(root, name));
  }
  return candidates;
}

async function pickExecutable(): Promise<string | undefined> {
  const picked = await vscode.window.showOpenDialog({
    canSelectMany: false,
    title: '选择 agui.exe',
    filters: process.platform === 'win32' ? { 可执行文件: ['exe'] } : undefined,
  });
  if (!picked || picked.length === 0) return undefined;
  const target = vscode.workspace.workspaceFolders?.length
    ? vscode.ConfigurationTarget.Workspace
    : vscode.ConfigurationTarget.Global;
  await settings().update('path', picked[0].fsPath, target);
  void vscode.window.showInformationMessage(`Agui 解释器已设置为：${picked[0].fsPath}`);
  return picked[0].fsPath;
}

/**
 * 解析 agui.exe 的位置。
 * @param interactive 为 true 时，找不到会弹窗并提供“选择文件 / 打开设置”操作。
 */
async function resolveExecutable(uri: vscode.Uri | undefined, interactive: boolean): Promise<string | undefined> {
  const configured = settings().get<string>('path', '').trim();
  const folder = uri ? vscode.workspace.getWorkspaceFolder(uri) : vscode.workspace.workspaceFolders?.[0];
  const base = folder?.uri.fsPath ?? (uri?.scheme === 'file' ? path.dirname(uri.fsPath) : process.cwd());

  if (configured) {
    const expanded = configured
      .replace(/\$\{workspaceFolder\}/gi, base)
      .replace(/\$\{workspaceRoot\}/gi, base);
    const looksLikePath = path.isAbsolute(expanded) || /[\\/]/.test(expanded);
    if (!looksLikePath) return expanded; // 交给 PATH 解析
    const absolute = path.isAbsolute(expanded) ? expanded : path.resolve(base, expanded);
    if (isFile(absolute)) return absolute;
    if (!interactive) return undefined;
    const choice = await vscode.window.showErrorMessage(
      `agui.path 指向的文件不存在：${absolute}`,
      '重新选择',
      '打开设置',
    );
    return handleExecutableChoice(choice);
  }

  for (const candidate of executableCandidates(uri)) {
    if (isFile(candidate)) return candidate;
  }

  if (!interactive) return undefined;
  const choice = await vscode.window.showErrorMessage(
    '未找到 agui.exe。请把 agui.path 指向解释器，或把 agui.exe 放在工作区根目录 / PATH 中。',
    '选择文件',
    '打开设置',
  );
  return handleExecutableChoice(choice);
}

async function handleExecutableChoice(choice: string | undefined): Promise<string | undefined> {
  if (choice === '选择文件' || choice === '重新选择') return pickExecutable();
  if (choice === '打开设置') {
    await vscode.commands.executeCommand('workbench.action.openSettings', 'agui.path');
  }
  return undefined;
}

async function selectExecutableCommand(): Promise<void> {
  const picked = await pickExecutable();
  if (!picked) {
    await vscode.commands.executeCommand('workbench.action.openSettings', 'agui.path');
  }
}

// ---------------------------------------------------------------------------
// 文档辅助
// ---------------------------------------------------------------------------

async function documentFor(uri?: vscode.Uri): Promise<vscode.TextDocument | undefined> {
  if (uri) {
    try {
      const doc = await vscode.workspace.openTextDocument(uri);
      if (doc.languageId === LANGUAGE_ID) return doc;
    } catch {
      // 打开失败时退回当前编辑器
    }
  }
  const editor = vscode.window.activeTextEditor;
  if (editor?.document.languageId === LANGUAGE_ID) return editor.document;
  void vscode.window.showWarningMessage('请先打开一个 .ag 文件。');
  return undefined;
}

async function saveDocument(doc: vscode.TextDocument): Promise<boolean> {
  if (!doc.isDirty) return true;
  return doc.save();
}

// ---------------------------------------------------------------------------
// 语法检查
// ---------------------------------------------------------------------------

export interface CheckIssue {
  /** 1 起始的行号 */
  line: number;
  /** 1 起始的列号 */
  column: number;
  message: string;
}

/**
 * 解析 `agui check` 的输出，兼容：
 *   test.ag：第 3 行第 5 列: 这里需要一个表达式
 *   test.ag 第3行 错误:不能除以零
 *   test.ag:3:5: message
 */
export function parseCheckOutput(output: string): CheckIssue[] {
  const issues: CheckIssue[] = [];
  const chinese = /第\s*(\d+)\s*行(?:\s*第\s*(\d+)\s*列)?\s*(?:[:：]|错误\s*[:：]?)\s*(.+)$/;
  const generic = /^(.+?):(\d+):(\d+):\s*(.+)$/;

  for (const raw of output.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith('✓') || line.startsWith('用法')) continue;

    let issue: CheckIssue | undefined;
    const match = chinese.exec(line);
    if (match) {
      issue = {
        line: Number(match[1]),
        column: match[2] ? Number(match[2]) : 1,
        message: match[3].trim(),
      };
    } else {
      const alt = generic.exec(line);
      if (alt) {
        issue = { line: Number(alt[2]), column: Number(alt[3]), message: alt[4].trim() };
      }
    }

    if (!issue || !issue.message || !Number.isFinite(issue.line)) continue;
    issue.line = Math.max(1, issue.line);
    issue.column = Math.max(1, issue.column);
    if (!issues.some((item) => item.line === issue?.line && item.column === issue?.column)) {
      issues.push(issue);
    }
  }
  return issues;
}

function isWordChar(ch: string): boolean {
  return /[\p{L}\p{N}_]/u.test(ch);
}

function makeDiagnostic(doc: vscode.TextDocument, issue: CheckIssue): vscode.Diagnostic {
  const lineIndex = Math.min(Math.max(issue.line - 1, 0), Math.max(doc.lineCount - 1, 0));
  const lineText = lineIndex < doc.lineCount ? doc.lineAt(lineIndex).text : '';
  let start = Math.min(Math.max(issue.column - 1, 0), Math.max(lineText.length, 0));
  if (start >= lineText.length && lineText.length > 0) start = lineText.length - 1;

  let end = start + 1;
  if (start < lineText.length && isWordChar(lineText[start])) {
    while (end < lineText.length && isWordChar(lineText[end])) end += 1;
  }

  const range = new vscode.Range(lineIndex, start, lineIndex, end);
  const diagnostic = new vscode.Diagnostic(range, issue.message, vscode.DiagnosticSeverity.Error);
  diagnostic.source = 'agui';
  diagnostic.code = 'syntax';
  return diagnostic;
}

interface ExecResult {
  stdout: string;
  stderr: string;
  code: number;
}

function execFileAsync(file: string, args: string[], options: cp.ExecFileOptions): Promise<ExecResult> {
  return new Promise<ExecResult>((resolve, reject) => {
    cp.execFile(
      file,
      args,
      { ...options, encoding: 'utf8', windowsHide: true, maxBuffer: 8 * 1024 * 1024 },
      (error, stdout, stderr) => {
        const code = (error as { code?: unknown } | null)?.code;
        if (error && typeof code === 'string') {
          reject(error); // ENOENT 等启动错误
          return;
        }
        resolve({
          stdout: String(stdout ?? ''),
          stderr: String(stderr ?? ''),
          code: typeof code === 'number' ? code : error ? 1 : 0,
        });
      },
    );
  });
}

class AguiLinter implements vscode.Disposable {
  private readonly collection = vscode.languages.createDiagnosticCollection('agui');
  private readonly timers = new Map<string, ReturnType<typeof setTimeout>>();
  private readonly versions = new Map<string, number>();
  private readonly disposables: vscode.Disposable[] = [];
  private warnedMissing = false;

  constructor() {
    this.disposables.push(
      vscode.workspace.onDidOpenTextDocument((doc) => this.schedule(doc, 500)),
      vscode.workspace.onDidChangeTextDocument((event) => {
        if (!settings().get<boolean>('checkOnType', true)) return;
        this.schedule(event.document, settings().get<number>('checkDelay', 700));
      }),
      vscode.workspace.onDidSaveTextDocument((doc) => {
        if (!settings().get<boolean>('checkOnSave', true)) return;
        this.schedule(doc, 80);
      }),
      vscode.workspace.onDidCloseTextDocument((doc) => {
        this.cancel(doc.uri);
        this.collection.delete(doc.uri);
      }),
      vscode.workspace.onDidChangeConfiguration((event) => {
        if (!event.affectsConfiguration('agui')) return;
        this.warnedMissing = false;
        this.collection.clear();
        for (const doc of vscode.workspace.textDocuments) {
          if (doc.languageId === LANGUAGE_ID) this.schedule(doc, 300);
        }
      }),
    );

    for (const doc of vscode.workspace.textDocuments) {
      if (doc.languageId === LANGUAGE_ID) this.schedule(doc, 800);
    }
  }

  /** 立即检查一个文档（供命令复用）。 */
  async checkNow(doc: vscode.TextDocument): Promise<CheckIssue[] | undefined> {
    const key = doc.uri.toString();
    const timer = this.timers.get(key);
    if (timer) {
      clearTimeout(timer);
      this.timers.delete(key);
    }
    return this.check(doc);
  }

  dispose(): void {
    for (const timer of this.timers.values()) clearTimeout(timer);
    this.timers.clear();
    for (const disposable of this.disposables) disposable.dispose();
    this.collection.dispose();
  }

  private schedule(doc: vscode.TextDocument, delay: number): void {
    if (doc.languageId !== LANGUAGE_ID || doc.uri.scheme !== 'file') return;
    const key = doc.uri.toString();
    const existing = this.timers.get(key);
    if (existing) clearTimeout(existing);
    this.timers.set(
      key,
      setTimeout(() => {
        this.timers.delete(key);
        void this.check(doc);
      }, Math.max(0, delay)),
    );
  }

  private cancel(uri: vscode.Uri): void {
    const key = uri.toString();
    const timer = this.timers.get(key);
    if (timer) {
      clearTimeout(timer);
      this.timers.delete(key);
    }
    this.versions.delete(key);
  }

  private async check(doc: vscode.TextDocument): Promise<CheckIssue[] | undefined> {
    if (doc.languageId !== LANGUAGE_ID || doc.uri.scheme !== 'file') return undefined;

    const executable = await resolveExecutable(doc.uri, false);
    if (!executable) {
      this.warnMissing();
      return undefined;
    }

    const key = doc.uri.toString();
    const version = (this.versions.get(key) ?? 0) + 1;
    this.versions.set(key, version);

    let dir = '';
    try {
      dir = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'agui-check-'));
      const tempFile = path.join(dir, 'check.ag');
      // 检查临时快照，这样未保存的编辑也能得到准确的诊断。
      await fs.promises.writeFile(tempFile, doc.getText(), 'utf8');

      const result = await execFileAsync(executable, ['check', tempFile], {
        cwd: workingDirectory(doc),
        timeout: 20000,
      });
      if (this.versions.get(key) !== version) return undefined;

      if (result.code === 0) {
        this.collection.set(doc.uri, []);
        return [];
      }

      const issues = parseCheckOutput(`${result.stdout}\n${result.stderr}`);
      if (issues.length === 0) {
        const text =
          [result.stdout, result.stderr]
            .map((part) => part.trim())
            .filter(Boolean)
            .join('\n') || `agui check 退出码 ${result.code}`;
        const diagnostic = new vscode.Diagnostic(
          new vscode.Range(0, 0, 0, 1),
          text,
          vscode.DiagnosticSeverity.Error,
        );
        diagnostic.source = 'agui';
        this.collection.set(doc.uri, [diagnostic]);
        return [{ line: 1, column: 1, message: text }];
      }

      this.collection.set(doc.uri, issues.map((issue) => makeDiagnostic(doc, issue)));
      return issues;
    } catch (err) {
      if ((err as { code?: unknown } | undefined)?.code === 'ENOENT') {
        this.warnMissing();
      } else {
        const diagnostic = new vscode.Diagnostic(
          new vscode.Range(0, 0, 0, 1),
          `无法运行 agui check：${errorText(err)}`,
          vscode.DiagnosticSeverity.Warning,
        );
        diagnostic.source = 'agui';
        this.collection.set(doc.uri, [diagnostic]);
      }
      return undefined;
    } finally {
      if (dir) {
        void fs.promises.rm(dir, { recursive: true, force: true }).catch(() => undefined);
      }
    }
  }

  private warnMissing(): void {
    if (this.warnedMissing) return;
    this.warnedMissing = true;
    void vscode.window
      .showWarningMessage(
        '未找到 agui.exe，语法检查已跳过。请在设置 `agui.path` 中指定解释器路径。',
        '设置路径',
      )
      .then((choice) => {
        if (choice === '设置路径') void vscode.commands.executeCommand('agui.selectExecutable');
      });
  }
}

// ---------------------------------------------------------------------------
// 终端与进程
// ---------------------------------------------------------------------------

interface TerminalSpec {
  exe: string;
  args: string[];
  cwd: string;
  /** 进程结束后是否保留终端（方便查看构建输出）。 */
  keepOpen: boolean;
  onExit?: (code: number) => void;
}

class AguiProcessTerminal implements vscode.Pseudoterminal {
  private readonly writeEmitter = new vscode.EventEmitter<string>();
  private readonly closeEmitter = new vscode.EventEmitter<number>();
  readonly onDidWrite = this.writeEmitter.event;
  readonly onDidClose = this.closeEmitter.event;

  private child: cp.ChildProcess | undefined;
  private finished = false;
  private readonly startedAt = Date.now();

  constructor(private readonly spec: TerminalSpec) {}

  open(): void {
    const { exe, args, cwd } = this.spec;
    this.write(`\x1b[2m${cwd}\x1b[0m\r\n`);
    this.write(`\x1b[36m> ${quoteShell(exe)} ${args.map(quoteShell).join(' ')}\x1b[0m\r\n\r\n`);

    let child: cp.ChildProcess;
    try {
      child = cp.spawn(exe, args, { cwd, windowsHide: true });
    } catch (err) {
      this.write(`\x1b[31m启动失败：${errorText(err)}\x1b[0m\r\n`);
      this.finish(1);
      return;
    }

    this.child = child;
    child.stdout?.on('data', (chunk: Buffer) => this.write(chunk.toString('utf8')));
    child.stderr?.on('data', (chunk: Buffer) =>
      this.write(`\x1b[31m${chunk.toString('utf8')}\x1b[0m`),
    );
    child.on('error', (err) => {
      this.write(`\x1b[31m启动失败：${errorText(err)}\x1b[0m\r\n`);
    });
    child.on('close', (code, signal) => {
      if (this.finished) return;
      this.finished = true;
      const elapsed = Date.now() - this.startedAt;
      if (signal) {
        this.write(`\r\n\x1b[33m进程被信号 ${signal} 结束（${elapsed} ms）\x1b[0m\r\n`);
      } else {
        const color = code === 0 ? '\x1b[32m' : '\x1b[31m';
        this.write(`\r\n${color}进程结束，退出码 ${code ?? '未知'}\x1b[0m（${elapsed} ms）\r\n`);
      }
      this.spec.onExit?.(code ?? 0);
      if (!this.spec.keepOpen) this.closeEmitter.fire(code ?? 0);
    });
  }

  close(): void {
    if (this.finished) return;
    this.finished = true;
    this.child?.kill();
  }

  handleInput(data: string): void {
    if (data.includes('\u0003')) this.close(); // Ctrl+C
  }

  private finish(code: number): void {
    this.spec.onExit?.(code);
    if (!this.spec.keepOpen) this.closeEmitter.fire(code);
  }

  private write(text: string): void {
    this.writeEmitter.fire(text.replace(/\r?\n/g, '\r\n'));
  }
}

function disposeTerminal(): void {
  if (activeTerminal) {
    const terminal = activeTerminal;
    activeTerminal = undefined;
    terminal.dispose();
  }
}

function startProcessTerminal(name: string, icon: string, spec: TerminalSpec): vscode.Terminal {
  disposeTerminal();
  const terminal = vscode.window.createTerminal({
    name,
    pty: new AguiProcessTerminal(spec),
    iconPath: new vscode.ThemeIcon(icon),
  });
  activeTerminal = terminal;
  terminal.show(false);
  return terminal;
}

// ---------------------------------------------------------------------------
// 命令实现
// ---------------------------------------------------------------------------

async function runScript(uri?: vscode.Uri): Promise<void> {
  const doc = await documentFor(uri);
  if (!doc) return;
  if (!(await saveDocument(doc))) {
    void vscode.window.showErrorMessage('保存失败，未运行脚本。');
    return;
  }

  const executable = await resolveExecutable(doc.uri, true);
  if (!executable) return;

  startProcessTerminal(`Agui · 运行 ${path.basename(doc.fileName)}`, 'play', {
    exe: executable,
    args: ['run', doc.uri.fsPath],
    cwd: workingDirectory(doc),
    keepOpen: false,
  });
}

async function checkScript(uri?: vscode.Uri): Promise<void> {
  const doc = await documentFor(uri);
  if (!doc) return;

  const issues = await linter?.checkNow(doc);
  if (!issues) return;

  if (issues.length === 0) {
    void vscode.window.showInformationMessage(`✓ 语法检查通过：${path.basename(doc.fileName)}`);
    return;
  }

  const first = issues[0];
  const editor = vscode.window.activeTextEditor;
  if (editor && editor.document === doc) {
    const position = new vscode.Position(first.line - 1, Math.max(first.column - 1, 0));
    editor.selection = new vscode.Selection(position, position);
    editor.revealRange(
      new vscode.Range(position, position),
      vscode.TextEditorRevealType.InCenterIfOutsideViewport,
    );
  }
  void vscode.window.showErrorMessage(
    `发现 ${issues.length} 个语法错误：第 ${first.line} 行 — ${first.message}`,
  );
}

async function buildScript(uri?: vscode.Uri): Promise<void> {
  const doc = await documentFor(uri);
  if (!doc) return;
  if (!(await saveDocument(doc))) {
    void vscode.window.showErrorMessage('保存失败，未打包。');
    return;
  }

  const executable = await resolveExecutable(doc.uri, true);
  if (!executable) return;

  const baseName = path.basename(doc.fileName, path.extname(doc.fileName));
  const outSetting = settings().get<string>('buildOutputDir', 'out');
  const outDir = path.isAbsolute(outSetting)
    ? outSetting
    : path.resolve(workingDirectory(doc), outSetting);
  try {
    await fs.promises.mkdir(outDir, { recursive: true });
  } catch (err) {
    void vscode.window.showErrorMessage(`无法创建输出目录：${errorText(err)}`);
    return;
  }

  const outFile = path.join(outDir, `${baseName}.exe`);
  const extra = splitArgs(settings().get<string>('buildOptions', ''));
  const args = ['build', doc.uri.fsPath, '-o', outFile, ...extra];

  startProcessTerminal(`Agui · 打包 ${baseName}`, 'package', {
    exe: executable,
    args,
    cwd: workingDirectory(doc),
    keepOpen: true,
    onExit: (code) => {
      if (code !== 0) {
        void vscode.window
          .showErrorMessage(`打包失败（退出码 ${code}），请查看“Agui · 打包”终端中的输出。`)
          .then(undefined, () => undefined);
        return;
      }
      void vscode.window
        .showInformationMessage(`打包完成：${outFile}`, '在资源管理器中显示', '打开输出目录')
        .then((choice) => {
          if (choice === '在资源管理器中显示') {
            void vscode.commands.executeCommand('revealFileInOS', vscode.Uri.file(outFile));
          } else if (choice === '打开输出目录') {
            void vscode.commands.executeCommand('vscode.openFolder', vscode.Uri.file(outDir), {
              forceNewWindow: true,
            });
          }
        });
    },
  });
}

// ---------------------------------------------------------------------------
// 补全 / 悬停
// ---------------------------------------------------------------------------

const KIND_MAP: Record<ApiKind, vscode.CompletionItemKind> = {
  widget: vscode.CompletionItemKind.Function,
  builtin: vscode.CompletionItemKind.Function,
  gui: vscode.CompletionItemKind.Function,
  keyword: vscode.CompletionItemKind.Keyword,
  event: vscode.CompletionItemKind.Event,
  property: vscode.CompletionItemKind.Property,
  method: vscode.CompletionItemKind.Method,
};

const KEYWORD_ALIASES: Record<string, string> = {
  如果: 'if',
  否则如果: 'elif',
  否则: 'else',
  当: 'while',
  对于: 'for',
  在: 'in',
  函数: 'func',
  返回: 'return',
  中断: 'break',
  继续: 'continue',
  监听: 'on',
  略过: 'pass',
  尝试: 'try',
  捕获: 'catch',
  导入: 'import',
  匿名: 'lambda',
  并且: 'and',
  且: 'and',
  或者: 'or',
  或: 'or',
  非: 'not',
  真: 'true',
  假: 'false',
  空: 'null',
};

function toCompletionItem(entry: ApiEntry): vscode.CompletionItem {
  const item = new vscode.CompletionItem(entry.name, KIND_MAP[entry.kind]);
  item.detail = entry.signature ?? (entry.kind === 'keyword' ? '关键字' : entry.kind);
  item.documentation = new vscode.MarkdownString(entry.doc);
  item.insertText = entry.snippet ? new vscode.SnippetString(entry.snippet) : entry.name;
  const rank = entry.kind === 'widget' ? '0' : entry.kind === 'builtin' ? '1' : '2';
  item.sortText = `${rank}_${entry.name}`;
  return item;
}

function createCompletionProvider(): vscode.Disposable {
  return vscode.languages.registerCompletionItemProvider(
    LANGUAGE_ID,
    {
      provideCompletionItems(document, position) {
        const linePrefix = document.lineAt(position).text.slice(0, position.character);

        // on 控件.<事件>
        if (/(?:on|监听)\s+[\p{L}\p{N}_]+\s*\.\s*[\p{L}\p{N}_]*$/u.test(linePrefix)) {
          return EVENTS.map((event) => {
            const item = new vscode.CompletionItem(event.name, vscode.CompletionItemKind.Event);
            item.detail = `事件 · ${event.controls}`;
            item.documentation = new vscode.MarkdownString(
              `${event.doc}\n\n支持控件：\`${event.controls}\``,
            );
            return item;
          });
        }

        // 任意控件的属性和方法
        if (/\.[\p{L}\p{N}_]*$/u.test(linePrefix)) {
          return [...PROPERTIES, ...METHODS].map((entry) => toCompletionItem(entry));
        }

        return [...WIDGETS, ...GUI_FUNCTIONS, ...BUILTINS, ...KEYWORDS].map((entry) =>
          toCompletionItem(entry),
        );
      },
    },
    '.',
  );
}

function createHoverProvider(): vscode.Disposable {
  return vscode.languages.registerHoverProvider(LANGUAGE_ID, {
    provideHover(document, position) {
      const range = document.getWordRangeAtPosition(position, /[\p{L}\p{N}_]+/u);
      if (!range) return undefined;
      const word = document.getText(range);

      const alias = KEYWORD_ALIASES[word];
      const entry = ALL_ENTRIES.find((item) => item.name === (alias ?? word));

      if (entry) {
        const contents = new vscode.MarkdownString();
        if (entry.signature) contents.appendCodeblock(entry.signature, 'agui');
        contents.appendMarkdown(entry.doc);
        if (alias) contents.appendMarkdown(`\n\n中文别名 → \`${alias}\``);
        return new vscode.Hover(contents, range);
      }

      const event = EVENTS.find((item) => item.name === word);
      if (event) {
        const contents = new vscode.MarkdownString(
          `**事件 \`${event.name}\`**\n\n${event.doc}\n\n支持控件：\`${event.controls}\``,
        );
        return new vscode.Hover(contents, range);
      }
      return undefined;
    },
  });
}

// ---------------------------------------------------------------------------
// 激活
// ---------------------------------------------------------------------------

export function activate(context: vscode.ExtensionContext): void {
  extensionRoot = context.extensionPath;
  linter = new AguiLinter();

  const disposables: vscode.Disposable[] = [
    linter,
    createCompletionProvider(),
    createHoverProvider(),
    vscode.commands.registerCommand('agui.run', (arg?: unknown) => runScript(asUri(arg))),
    vscode.commands.registerCommand('agui.check', (arg?: unknown) => checkScript(asUri(arg))),
    vscode.commands.registerCommand('agui.build', (arg?: unknown) => buildScript(asUri(arg))),
    vscode.commands.registerCommand('agui.selectExecutable', () => selectExecutableCommand()),
    vscode.window.onDidCloseTerminal((terminal) => {
      if (terminal === activeTerminal) activeTerminal = undefined;
    }),
  ];

  context.subscriptions.push(...disposables);
}

export function deactivate(): void {
  disposeTerminal();
  linter = undefined;
}

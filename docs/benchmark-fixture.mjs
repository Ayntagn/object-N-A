import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {mkdirSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';

const folder = process.argv[2];
assert(folder, 'Pass a new disposable directory for the generated vault.');
const target = resolve(folder);
const title = "# 计算系统笔记\n\n";
const prose = "本节围绕虚拟化、并发和持久性。通过学习这几个概念，可以理解操作系统如何分配资源、处理内存紧张，以及在多个进程之间协调工作。本节围绕虚拟化、并发和持久性。通过学习这几个概念，可以理解操作系统如何分配资源、处理内存紧张，以及在多个进程之间协调工作。\n\n　　记录一个新的问题。比较多种输入时的输出内容，核对光标、渲染和界面的资源使用。记录一个新的问题。比较多种输入时的输出内容，核对光标、渲染和界面的资源使用。\n\n";
const code = "```c\n#include <stdio.h>\n#include <stdlib.h>\nstatic int synthetic_counter(int argc, char **argv) {\n    if (argc != 2) {\n        fprintf(stderr, \"usage: cpu <string>\\n\");\n        return EXIT_FAILURE;\n    }\n    puts(argv[1]);\n    return 0;\n}\n```\n\n";

function section(number) {
  return '## 练习 ' + number + '\n\n' + prose + (number % 4 === 1 ? code : '');
}
const source = title + Array.from({length:180}, (_, index) => section(index + 1)).join('');
const digest = createHash('sha256').update(source).digest('hex');
assert.equal(Buffer.byteLength(source), 122314);
assert.equal(digest, '1a762007bd0d273af9d09e5a8791e2f72702fbeb636ea18d355148a2318fa0de');
mkdirSync(target);
writeFileSync(join(target, 'Computation Notes.md'), source, {flag:'wx'});
for (let index = 0; index < 4500; index++) {
  const name = 'Reference ' + String(index).padStart(5, '0') + '.md';
  const note = '# Reference ' + index + '\n\nA short synthetic reference.\n';
  writeFileSync(join(target, name), note, {flag:'wx'});
}
console.log(JSON.stringify({files:4501, activeNoteBytes:Buffer.byteLength(source), activeNoteSHA256:digest}));

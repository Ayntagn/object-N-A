# object-N/A 与 Obsidian 的性能对比

测试日期为 2026 年 11 月 22 日。使用 Apple M5 Mac（24 GiB 内存）、macOS 27.0.1（26A434）、object-N/A v0.4.0（构建 187）和 Obsidian 1.15.1（安装器 1.12.7）。结论限于这台机器、这些版本和下面的分栏场景。

## 测量方法

- 每次使用独立配置和合成 Vault 副本，包含 4,501 篇笔记。打开同一份 122,314 字节的 Markdown，启用源码和阅读分栏及行号，关闭拼写检查。窗口固定为 1,300 × 840 点。每组交换先运行的应用，系统文件缓存保留。
- 启动计时从发起新进程启动到辅助功能首次读到该笔记标题的窗口结束。Obsidian 的首窗时间在各轮有明显波动。下表保留原始时间，主页主要汇报进入工作区后的资源读数。
- 窗口出现后等待约 9 秒。空闲和编辑各以约 750 ms 间隔采样 12 次，每段约 8.25 秒。通过 macOS 的资源进程组及 `proc_pid_rusage` 合计物理内存与累计 CPU，包含 WebKit 或 Electron 辅助进程。内存的峰值取这一轮采样最高值，CPU 的 100% 对应一个核心。
- 在源码分栏通过键盘写入同样的 37 字节并保存，编辑窗口含停笔后的处理时间。每轮校验窗口、前台状态、渲染辅助进程和保存结果。六次保存后的笔记均为 122,351 字节，SHA-256 一致。

## 逐轮数据

| 组次 | 启动顺序 | 应用 | 首个笔记窗口 | 空闲物理内存 | 空闲平均 CPU | 编辑物理内存 | 编辑内存采样峰值 | 编辑平均 CPU |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | object-N/A → Obsidian | object-N/A | 0.501 s | 332.2 MiB | 1.13% | 386.6 MiB | 425.8 MiB | 17.72% |
| 1 | object-N/A → Obsidian | Obsidian | 4.480 s | 554.0 MiB | 2.04% | 628.9 MiB | 691.3 MiB | 24.32% |
| 2 | Obsidian → object-N/A | Obsidian | 0.762 s | 492.4 MiB | 2.09% | 567.2 MiB | 629.6 MiB | 25.37% |
| 2 | Obsidian → object-N/A | object-N/A | 0.503 s | 331.1 MiB | 1.16% | 402.6 MiB | 442.2 MiB | 17.90% |
| 3 | object-N/A → Obsidian | object-N/A | 0.505 s | 331.5 MiB | 1.20% | 385.6 MiB | 426.3 MiB | 18.04% |
| 3 | object-N/A → Obsidian | Obsidian | 0.733 s | 487.1 MiB | 2.33% | 560.0 MiB | 621.3 MiB | 24.47% |

每轮的物理内存读数为空闲或编辑段的中位数。README 再取三个轮次的中位数。

## 夹具校验

运行下面的命令，在尚未存在的目录中生成同一份合成 Vault。 [夹具生成器](benchmark-fixture.mjs)

```sh
node docs/benchmark-fixture.mjs /tmp/object-na-benchmark-vault
```

- 输入笔记 SHA-256 为 `1a762007bd0d273af9d09e5a8791e2f72702fbeb636ea18d355148a2318fa0de`。
- 保存笔记 SHA-256 为 `82f6f3be8b40a2b80e50ef20ef66f22bfe799c66f6525ee50d359afb3afb60b3`。

# Vault 目录变更范围

`object-na.local/vaultdelta` 合并递归文件监听给出的目录提示。`Hints` 接受以 `/` 分隔的相对目录路径，去重后只保留最上层的受影响目录。根目录提示、无效路径或过大的提示批次会请求全量刷新。`Scope.Merge` 可合并刷新期间到达的提示，已有的 `Scope` 可以交给后台读取。

Vault 调用方用同一套文件过滤规则读取受影响的目录，并复制改动路径上的父级节点。已经移走或无法定位的目录、刷新期间变化的目录和发生变化的祖先目录会触发全量扫描。提示目录在 macOS 或 Linux 上包含硬链接时，全量刷新会同时更新其他目录中的别名。Markdown 元数据继续由现有的文件变更戳和全 Vault 链接解析核对。 [目录刷新实现](../../internal/core/workspace/sidebar_delta.go)

下列命令在仓库根目录运行。基准对照同一套 4,501 篇合成笔记中的单篇保存与改名，计入文件树和 Markdown 元数据两部分刷新。

```sh
CGO_ENABLED=0 go test -run TestSidebarDeltaMatchesFullRefresh ./internal/core/workspace
CGO_ENABLED=0 go test -run '^$' -bench '^BenchmarkVaultRefresh$' -benchtime=500ms -count=3 ./internal/core/workspace
```

# 正文短查询候选索引

`object-na.local/shortsearch` 使用 SQLite FTS5 持久化正文候选项。正文去掉一个开头的 BOM，全文做 Unicode case folding 和 NFC 归一化，再按文档去重相邻码点对。FTS5 保存词元对应的文档编号。SQLite 的表另存文件路径标识和调用方给出的版本。

## 调用方式

- `Open(ctx, filename)` 在已有缓存目录下打开或创建 SQLite 文件。路径可包含 `CacheVersion`，它由模式和 Unicode 表版本组成。SQLite 文件内也记录应用 ID 和模式版本。收到 `ErrIncompatible` 时由缓存所有者选择新路径或重建。
- `Versions(ctx)` 返回各路径的现存版本。`Update(ctx, docs, removedPaths)` 在同一事务中先移除路径，再写入文档新版本。路径和版本当作标识原样保存。调用方以小批量传入已保存正文，SQLite 只写派生缓存及其事务文件。
- `Lookup(ctx, query)` 接受调用方裁边后的原始查询。它将归一化后的前 32 个不同码点对以 AND 组合，返回同一 SQLite 快照中的全部版本和候选路径。查询不足两个归一化码点时返回 `ErrUnfilterable`。
- 调用方应用自己的字素门槛、元数据匹配和排序规则。只有路径出现在 `Versions` 中、当前文件版本可信且一致、并且路径不在 `Matches` 中时，才可跳过正文读取。候选路径、版本未验证的文件和索引错误均交给原精确匹配器处理。
- `Close()` 释放数据库连接。改变词元编码或 Unicode 归一化语义时，需要更新模式版本，让已有缓存按新规则重建。

## 测试和基准

测试覆盖 Unicode 候选、长查询、持久化替换与移除、事务回滚和错误回退。基准在 192 篇合成文档上测量稀疏词、常用词和零命中词的候选查询。

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go test -run '^$' -bench BenchmarkLookup -benchmem
```

Go 驱动固定为 `github.com/ncruces/go-sqlite3 v0.38.0`，归一化复用 `golang.org/x/text`。 [依赖登记](../../docs/THIRD_PARTY.md)

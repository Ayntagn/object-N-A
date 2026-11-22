# vectorsearch

Go 1.24+ 自用模块。保存向量时以 `Norm` 计算一次范数，查询时用 `CosineTopK` 对 `float32` 向量逐行算精确余弦分数。排序保留输入次序的同分行为，工作内存随 K 增长。调用方可沿用自己的向量布局和模型。

```sh
CGO_ENABLED=0 go test ./...
```

# pngscale

`pngscale` 是独立的 Go 1.23+ 模块，从 `io.Reader` 逐扫描行解码 PNG，同时进行面积平均（Box）缩小。支持标准 PNG 的颜色类型、位深、透明度及 Adam7 交错。APNG 读取默认图像。面积平均直接使用样本值，色彩管理可由调用方处理。

```go
import "object-na.local/pngscale"

img, err := pngscale.Decode(reader, 1024, 768)
```

- 每个目标轴必须介于 1 和原尺寸之间。保持某一轴尺寸时传入原值。
- 1、2、4、8 位源图得到紧致的 `*image.RGBA`，16 位源图得到 `*image.RGBA64`。面积平均使用预乘 alpha，舍入发生在输出时。
- 成功前验证 PNG 块 CRC、zlib Adler-32、扫描行数量和 IEND。读取位置停在 IEND 后。
- `DecodeWithOptions` 可按调用设置 `MaxWorkingBytes` 和 `MaxSourcePixels`。扫描线、累计值和输出缓冲默认合计最多 256 MiB。该预算之外还有固定解码器状态和调用方持有的输入。超限返回 `ErrLimits`。

非交错图像复用两条源扫描线和至多两行目标宽度的累计值。Adam7 将各遍像素计入目标尺寸的累计缓冲。两种模式均使用 Go 标准库解压。解滤波沿用 Go PNG 的算法与 BSD-3-Clause 许可。 [PNG 规范](https://www.w3.org/TR/png-3/) · [Go Paeth 实现](https://go.dev/src/image/png/paeth.go) · [许可证](LICENSE)

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 GOMAXPROCS=1 go test -run '^$' -bench BenchmarkDecodeBox -benchmem -count=5
```

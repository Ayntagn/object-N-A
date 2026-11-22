# jpegscale

独立 Go 模块 `object-na.local/jpegscale`，最低 Go 1.24。支持 `CGO_ENABLED=0`，在受支持的架构使用 Go 汇编 SIMD。`-tags noasm` 选用纯 Go 实现。

```go
import "object-na.local/jpegscale"

preview, err := jpegscale.Decode(jpegBytes, jpegscale.Options{
    ScaleDenom: 4,
    MaxPixels:  32 << 20,
})
```

每次调用独立选择 `ScaleDenom`。零值及 `1` 返回标准库原尺寸图像，`2`、`4`、`8` 选择缩小 IDCT，输出每边向上取整。图像保持文件中的方向，EXIF 旋转由调用方处理。导入后，`image.Decode` 继续使用标准库注册的 JPEG 解码器。

解码先检查源图尺寸、JPEG 帧与扫描覆盖、EOI，再调用 `image/jpeg.Decode` 完整读取扫描数据。选择缩小时，后端直接读取同一份压缩字节，用更小的 IDCT 写出目标像素。校验仍会分配原尺寸图像，渐进 JPEG 还要分配完整系数。调用期间保持输入不变。`MaxPixels` 零值允许 33,554,432 个源像素，另受架构和解码器容量约束。

返回的 `ErrUnsupported` 表示原图已通过校验，缩小后端遇到扫描恢复、格式兼容或输出一致性问题。调用方可用 `errors.Is` 识别并采用标准解码后的缩放路径。标准库的格式限制同样适用，例如 Go 1.27.3 会拒绝实测中的一张带逐块重启间隔的渐进 JPEG。渐进 JPEG 允许在各分量的 DC 扫描完整后以 EOI 结束，因此合法省略的 AC 频段或低位不能从文件本身区分是否源于人为缩短。读取在第一个完整图像的 EOI 处结束。 [JPEG T.81](https://www.w3.org/Graphics/JPEG/itu-t81.pdf) · [libjpeg-turbo 渐进扫描规则](https://github.com/libjpeg-turbo/libjpeg-turbo/blob/main/src/jcmaster.c)

## 上游

`internal/jpegn` 保存固定版本 v0.7.1 的解码代码与 MIT 许可。内部入口接收同一份字节切片，移除了 `image.RegisterFormat`、编码器及公共 EXIF 辅助 API。熵解码遇到缺失的 Huffman 码、值位或重启标记时返回错误，保留 SIMD 和纯 Go IDCT 内核。Go 模块源校验为 `h1:1YpwscJuMUkVkyULYt3INrl+4VsZW/pD2FYO5TyzTno=`。 [jpegn v0.7.1](https://github.com/gen2brain/jpegn/tree/v0.7.1) · [MIT 许可](internal/jpegn/LICENSE)

## 本地验证

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go test -tags noasm ./...
CGO_ENABLED=0 go test -run '^$' -bench '^BenchmarkDecodePreview$' -benchmem -cpu=1 -benchtime=300ms -count=3
```

测试 JPEG 是 37×29 的合成渐变图，经 libjpeg-turbo `cjpeg` 分别编码成带重启标记的基线、普通渐进及分量独立扫描的 JPEG。另有 Go 生成的合成基线图用于截断测试和基准。基准对比 4096×4096 源图生成 1024×1024 RGBA，包含完整 RGBA 中间图、直接平均 YCbCr 平面与缩小 IDCT 三种像素处理路径。三种路径的取整与色度采样不同，输出像素值可能有差异。

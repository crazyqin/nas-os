package aiphoto

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
)

// Upscaler 照片超分辨率放大器
type Upscaler struct {
	opts *UpscaleOptions
}

// NewUpscaler 创建超分辨率放大器
func NewUpscaler(opts *UpscaleOptions) *Upscaler {
	if opts == nil {
		opts = DefaultUpscaleOptions()
	}
	if opts.ScaleFactor == 0 {
		opts.ScaleFactor = 2
	}
	if opts.Model == "" {
		opts.Model = "realesrgan"
	}
	return &Upscaler{opts: opts}
}

// Upscale 对图像执行超分辨率放大
func (u *Upscaler) Upscale(ctx context.Context, src image.Image) (image.Image, error) {
	if src == nil {
		return nil, fmt.Errorf("源图像不能为 nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	bounds := src.Bounds()
	scale := u.opts.ScaleFactor
	newWidth := bounds.Dx() * scale
	newHeight := bounds.Dy() * scale

	var dst image.Image

	switch u.opts.Model {
	case "realesrgan":
		dst = u.realESRGANUpscale(src, newWidth, newHeight)
	case "esrgan":
		dst = u.esrganUpscale(src, newWidth, newHeight)
	default: // lanczos
		dst = u.lanczosUpscale(src, newWidth, newHeight)
	}

	return dst, nil
}

// realESRGANUpscale Real-ESRGAN 风格超分辨率（简化：双三次插值 + 锐化）
func (u *Upscaler) realESRGANUpscale(src image.Image, newWidth, newHeight int) image.Image {
	// 先用双三次插值放大
	dst := u.bicubicUpscale(src, newWidth, newHeight)

	// 应用自适应锐化增强细节
	dst = u.adaptiveSharpen(dst)

	// 如果启用了去噪，应用轻度去噪
	if u.opts.Denoise {
		denoiser := NewDenoiser(&DenoiseOptions{
			Strength:       0.2,
			PreserveDetail: true,
			Algorithm:      "nlm",
		})
		denoised, err := denoiser.Denoise(context.Background(), dst)
		if err == nil {
			dst = denoised
		}
	}

	return dst
}

// esrganUpscale ESRGAN 风格超分辨率（简化：双三次插值 + 适度锐化）
func (u *Upscaler) esrganUpscale(src image.Image, newWidth, newHeight int) image.Image {
	dst := u.bicubicUpscale(src, newWidth, newHeight)
	dst = u.unsharpMask(dst, 1.5, 0.5)
	return dst
}

// lanczosUpscale Lanczos 重采样放大
func (u *Upscaler) lanczosUpscale(src image.Image, newWidth, newHeight int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	xScale := float64(bounds.Dx()) / float64(newWidth)
	yScale := float64(bounds.Dy()) / float64(newHeight)
	a := 3 // Lanczos 参数

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := float64(x)*xScale + float64(bounds.Min.X)
			srcY := float64(y)*yScale + float64(bounds.Min.Y)

			var sumR, sumG, sumB, sumWeight float64

			xInt := int(math.Floor(srcX))
			yInt := int(math.Floor(srcY))

			for ky := yInt - a + 1; ky <= yInt+a; ky++ {
				for kx := xInt - a + 1; kx <= xInt+a; kx++ {
					// 边界处理
					cx := clampInt(kx, bounds.Min.X, bounds.Max.X-1)
					cy := clampInt(ky, bounds.Min.Y, bounds.Max.Y-1)

					// Lanczos 权重
					dx := srcX - float64(kx)
					dy := srcY - float64(ky)
					w := lanczosKernel(dx, a) * lanczosKernel(dy, a)

					r, g, b, _ := src.At(cx, cy).RGBA()
					sumWeight += w
					sumR += w * float64(r)
					sumG += w * float64(g)
					sumB += w * float64(b)
				}
			}

			if sumWeight > 0 {
				dst.Set(x, y, color.RGBA{
					R: clampUint8(int(sumR/sumWeight) >> 8),
					G: clampUint8(int(sumG/sumWeight) >> 8),
					B: clampUint8(int(sumB/sumWeight) >> 8),
					A: 255,
				})
			}
		}
	}

	return dst
}

// bicubicUpscale 双三次插值放大
func (u *Upscaler) bicubicUpscale(src image.Image, newWidth, newHeight int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	xScale := float64(bounds.Dx()) / float64(newWidth)
	yScale := float64(bounds.Dy()) / float64(newHeight)

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := float64(x)*xScale + float64(bounds.Min.X)
			srcY := float64(y)*yScale + float64(bounds.Min.Y)

			xInt := int(math.Floor(srcX))
			yInt := int(math.Floor(srcY))
			xFrac := srcX - float64(xInt)
			yFrac := srcY - float64(yInt)

			var sumR, sumG, sumB, sumW float64

			for ky := -1; ky <= 2; ky++ {
				for kx := -1; kx <= 2; kx++ {
					cx := clampInt(xInt+kx, bounds.Min.X, bounds.Max.X-1)
					cy := clampInt(yInt+ky, bounds.Min.Y, bounds.Max.Y-1)

					weight := cubicWeight(float64(kx)-xFrac) * cubicWeight(float64(ky)-yFrac)

					r, g, b, _ := src.At(cx, cy).RGBA()
					sumW += weight
					sumR += weight * float64(r)
					sumG += weight * float64(g)
					sumB += weight * float64(b)
				}
			}

			if sumW > 0 {
				dst.Set(x, y, color.RGBA{
					R: clampUint8(int(sumR/sumW) >> 8),
					G: clampUint8(int(sumG/sumW) >> 8),
					B: clampUint8(int(sumB/sumW) >> 8),
					A: 255,
				})
			}
		}
	}

	return dst
}

// adaptiveSharpen 自适应锐化
func (u *Upscaler) adaptiveSharpen(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	// 锐化核
	kernel := [3][3]float64{
		{0, -1, 0},
		{-1, 5, -1},
		{0, -1, 0},
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if x == bounds.Min.X || x == bounds.Max.X-1 || y == bounds.Min.Y || y == bounds.Max.Y-1 {
				dst.Set(x, y, src.At(x, y))
				continue
			}

			var sumR, sumG, sumB float64
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					r, g, b, _ := src.At(x+kx, y+ky).RGBA()
					w := kernel[ky+1][kx+1]
					sumR += w * float64(r)
					sumG += w * float64(g)
					sumB += w * float64(b)
				}
			}

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(sumR) >> 8),
				G: clampUint8(int(sumG) >> 8),
				B: clampUint8(int(sumB) >> 8),
				A: 255,
			})
		}
	}

	return dst
}

// unsharpMask USM 锐化
func (u *Upscaler) unsharpMask(src image.Image, radius float64, amount float64) image.Image {
	// 简化实现：使用 3x3 高斯模糊后的差值
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	// 高斯模糊核
	gaussian := [3][3]float64{
		{1.0 / 16, 2.0 / 16, 1.0 / 16},
		{2.0 / 16, 4.0 / 16, 2.0 / 16},
		{1.0 / 16, 2.0 / 16, 1.0 / 16},
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if x == bounds.Min.X || x == bounds.Max.X-1 || y == bounds.Min.Y || y == bounds.Max.Y-1 {
				dst.Set(x, y, src.At(x, y))
				continue
			}

			var blurR, blurG, blurB float64
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					r, g, b, _ := src.At(x+kx, y+ky).RGBA()
					w := gaussian[ky+1][kx+1]
					blurR += w * float64(r)
					blurG += w * float64(g)
					blurB += w * float64(b)
				}
			}

			origR, origG, origB, _ := src.At(x, y).RGBA()
			// sharpened = original + amount * (original - blur)
			newR := float64(origR) + amount*(float64(origR)-blurR)
			newG := float64(origG) + amount*(float64(origG)-blurG)
			newB := float64(origB) + amount*(float64(origB)-blurB)

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(newR) >> 8),
				G: clampUint8(int(newG) >> 8),
				B: clampUint8(int(newB) >> 8),
				A: 255,
			})
		}
	}

	return dst
}

// lanczosKernel Lanczos 核函数
func lanczosKernel(x float64, a int) float64 {
	if x == 0 {
		return 1
	}
	if x < -float64(a) || x > float64(a) {
		return 0
	}
	return float64(a) * math.Sin(math.Pi*x) * math.Sin(math.Pi*x/float64(a)) / (math.Pi * math.Pi * x * x)
}

// cubicWeight 双三次插值权重函数 (Catmull-Rom)
func cubicWeight(x float64) float64 {
	x = math.Abs(x)
	if x <= 1 {
		return 1.5*x*x*x - 2.5*x*x + 1
	}
	if x < 2 {
		return -0.5*x*x*x + 2.5*x*x - 4*x + 2
	}
	return 0
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

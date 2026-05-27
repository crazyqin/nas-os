package aiphoto

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
)

// Denoiser 照片去噪器
type Denoiser struct {
	opts *DenoiseOptions
}

// NewDenoiser 创建去噪器
func NewDenoiser(opts *DenoiseOptions) *Denoiser {
	if opts == nil {
		opts = DefaultDenoiseOptions()
	}
	if opts.Algorithm == "" {
		opts.Algorithm = "nlm"
	}
	if opts.Strength == 0 {
		opts.Strength = 0.5
	}
	return &Denoiser{opts: opts}
}

// Denoise 对图像执行去噪处理
func (d *Denoiser) Denoise(ctx context.Context, src image.Image) (image.Image, error) {
	if src == nil {
		return nil, fmt.Errorf("源图像不能为 nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	switch d.opts.Algorithm {
	case "bilateral":
		d.bilateralFilter(src, dst)
	case "bm3d":
		d.bm3dFilter(src, dst)
	default: // nlm
		d.nlmFilter(src, dst)
	}

	return dst, nil
}

// nlmFilter Non-Local Means 去噪（简化版）
func (d *Denoiser) nlmFilter(src image.Image, dst *image.RGBA) {
	bounds := src.Bounds()

	// 搜索窗口和补丁大小
	patchSize := 3
	searchRadius := 5
	hParam := float64(10.0) * d.opts.Strength // h 参数，控制滤波强度

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var sumR, sumG, sumB, sumWeight float64

			// 在搜索窗口内寻找相似补丁
			for sy := maxInt(y-searchRadius, bounds.Min.Y); sy <= minInt(y+searchRadius, bounds.Max.Y-1); sy++ {
				for sx := maxInt(x-searchRadius, bounds.Min.X); sx <= minInt(x+searchRadius, bounds.Max.X-1); sx++ {
					// 计算补丁距离
					dist := patchDistance(src, x, y, sx, sy, patchSize, bounds)

					// 计算权重
					weight := math.Exp(-dist / (hParam * hParam))

					r, g, b, _ := src.At(sx, sy).RGBA()
					sumWeight += weight
					sumR += weight * float64(r)
					sumG += weight * float64(g)
					sumB += weight * float64(b)
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
}

// bilateralFilter 双边滤波去噪
func (d *Denoiser) bilateralFilter(src image.Image, dst *image.RGBA) {
	bounds := src.Bounds()
	sigmaS := 3.0 + d.opts.Strength*5.0  // 空间 sigma
	sigmaR := 10.0 + d.opts.Strength*40.0 // 范围 sigma
	radius := int(math.Ceil(2 * sigmaS))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r0, g0, b0, _ := src.At(x, y).RGBA()
			var sumR, sumG, sumB, sumW float64

			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx, ny := x+dx, y+dy
					if nx < bounds.Min.X || nx >= bounds.Max.X || ny < bounds.Min.Y || ny >= bounds.Max.Y {
						continue
					}

					r1, g1, b1, _ := src.At(nx, ny).RGBA()

					// 空间权重
					spatialDist := math.Sqrt(float64(dx*dx + dy*dy))
					spatialWeight := math.Exp(-spatialDist * spatialDist / (2 * sigmaS * sigmaS))

					// 范围权重
					colorDist := math.Sqrt(float64((r0-r1)*(r0-r1)+(g0-g1)*(g0-g1)+(b0-b1)*(b0-b1))) / 256.0
					rangeWeight := math.Exp(-colorDist * colorDist / (2 * sigmaR * sigmaR))

					weight := spatialWeight * rangeWeight
					sumR += weight * float64(r1)
					sumG += weight * float64(g1)
					sumB += weight * float64(b1)
					sumW += weight
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
}

// bm3dFilter BM3D 去噪（简化版：使用分块 NLM + 硬阈值）
func (d *Denoiser) bm3dFilter(src image.Image, dst *image.RGBA) {
	// BM3D 简化实现：先做 NLM，再对结果做硬阈值小波去噪
	d.nlmFilter(src, dst)
	d.hardThresholdWavelet(dst, d.opts.Strength)
}

// hardThresholdWavelet 硬阈值小波去噪（简化）
func (d *Denoiser) hardThresholdWavelet(img *image.RGBA, strength float64) {
	bounds := img.Bounds()
	threshold := uint32(uint32(30.0*strength) << 8)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// 简化：对低于阈值的高频分量归零
			centerR, centerG, centerB, _ := img.At(x, y).RGBA()
			if x > bounds.Min.X && x < bounds.Max.X-1 && y > bounds.Min.Y && y < bounds.Max.Y-1 {
				// 计算与邻域均值的差（模拟高频分量）
				avgR, avgG, avgB := neighborhoodMean(img, x, y, bounds)
				diffR := int(centerR) - int(avgR)
				diffG := int(centerG) - int(avgG)
				diffB := int(centerB) - int(avgB)

				if absInt(diffR) < int(threshold) {
					r = avgR
				}
				if absInt(diffG) < int(threshold) {
					g = avgG
				}
				if absInt(diffB) < int(threshold) {
					b = avgB
				}
			}
			img.Set(x, y, color.RGBA{
				R: clampUint8(int(r) >> 8),
				G: clampUint8(int(g) >> 8),
				B: clampUint8(int(b) >> 8),
				A: clampUint8(int(a) >> 8),
			})
		}
	}
}

// neighborhoodMean 计算 3x3 邻域均值
func neighborhoodMean(img *image.RGBA, x, y int, bounds image.Rectangle) (uint32, uint32, uint32) {
	var sumR, sumG, sumB uint64
	count := 0

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
				r, g, b, _ := img.At(nx, ny).RGBA()
				sumR += uint64(r)
				sumG += uint64(g)
				sumB += uint64(b)
				count++
			}
		}
	}

	if count == 0 {
		return 0, 0, 0
	}

	return uint32(sumR / uint64(count)),
		uint32(sumG / uint64(count)),
		uint32(sumB / uint64(count))
}

// patchDistance 计算两个补丁之间的欧氏距离
func patchDistance(src image.Image, x1, y1, x2, y2, patchSize int, bounds image.Rectangle) float64 {
	half := patchSize / 2
	var dist float64
	count := 0

	for dy := -half; dy <= half; dy++ {
		for dx := -half; dx <= half; dx++ {
			px1, py1 := x1+dx, y1+dy
			px2, py2 := x2+dx, y2+dy

			if px1 < bounds.Min.X || px1 >= bounds.Max.X || py1 < bounds.Min.Y || py1 >= bounds.Max.Y {
				continue
			}
			if px2 < bounds.Min.X || px2 >= bounds.Max.X || py2 < bounds.Min.Y || py2 >= bounds.Max.Y {
				continue
			}

			r1, g1, b1, _ := src.At(px1, py1).RGBA()
			r2, g2, b2, _ := src.At(px2, py2).RGBA()

			dr := float64(r1-r2) / 256.0
			dg := float64(g1-g2) / 256.0
			db := float64(b1-b2) / 256.0

			dist += dr*dr + dg*dg + db*db
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return dist / float64(count)
}

// 辅助函数

func clampUint8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

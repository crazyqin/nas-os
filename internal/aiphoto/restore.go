package aiphoto

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
)

// Restorer 照片修复器
type Restorer struct {
	opts *RestoreOptions
}

// NewRestorer 创建照片修复器
func NewRestorer(opts *RestoreOptions) *Restorer {
	if opts == nil {
		opts = DefaultRestoreOptions()
	}
	return &Restorer{opts: opts}
}

// Restore 对图像执行修复处理（老照片、划痕、污渍）
func (r *Restorer) Restore(ctx context.Context, src image.Image) (image.Image, error) {
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

	// 复制源图像到目标
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}

	// 1. 修复划痕
	if r.opts.RepairScratches {
		r.repairScratches(dst, bounds)
	}

	// 2. 修复污渍
	if r.opts.RepairStains {
		r.repairStains(dst, bounds)
	}

	// 3. 人脸增强
	if r.opts.EnhanceFace {
		r.enhanceFace(dst, bounds)
	}

	// 4. 全局色彩和对比度修复
	r.restoreColorContrast(dst, bounds)

	return dst, nil
}

// detectScratch 检测划痕（垂直或水平连续暗线）
func (r *Restorer) detectScratch(src *image.RGBA, x, y int, bounds image.Rectangle) bool {
	if x < bounds.Min.X+2 || x >= bounds.Max.X-2 || y < bounds.Min.Y+2 || y >= bounds.Max.Y-2 {
		return false
	}

	centerR, centerG, centerB, _ := src.At(x, y).RGBA()
	centerGray := (centerR*299 + centerG*587 + centerB*114) / 1000

	// 检查是否比周围暗
	avgGray := uint32(0)
	count := 0
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
				r, g, b, _ := src.At(nx, ny).RGBA()
				avgGray += (r*299 + g*587 + b*114) / 1000
				count++
			}
		}
	}

	if count == 0 {
		return false
	}
	avgGray /= uint32(count)

	// 划痕检测：中心比周围暗很多且宽度较窄
	darkThreshold := uint32(60)
	return centerGray+darkThreshold < avgGray
}

// repairScratches 修复划痕
func (r *Restorer) repairScratches(dst *image.RGBA, bounds image.Rectangle) {
	// 用形态学方法修复：检测细长暗线并用周围像素替换
	for y := bounds.Min.Y + 2; y < bounds.Max.Y-2; y++ {
		for x := bounds.Min.X + 2; x < bounds.Max.X-2; x++ {
			if r.detectScratch(dst, x, y, bounds) {
				// 检查是否为连续划痕的一部分（垂直或水平）
				isVerticalScratch := r.detectScratch(dst, x, y-1, bounds) || r.detectScratch(dst, x, y+1, bounds)
				isHorizontalScratch := r.detectScratch(dst, x-1, y, bounds) || r.detectScratch(dst, x+1, y, bounds)

				if isVerticalScratch || isHorizontalScratch {
					// 用周围非划痕像素的均值替换
					r.replaceWithNeighborhood(dst, x, y, bounds)
				}
			}
		}
	}
}

// replaceWithNeighborhood 用邻域像素替换
func (r *Restorer) replaceWithNeighborhood(dst *image.RGBA, x, y int, bounds image.Rectangle) {
	var sumR, sumG, sumB uint64
	count := 0
	radius := 3

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
				// 跳过也是划痕的像素
				if r.detectScratch(dst, nx, ny, bounds) {
					continue
				}
				rv, gv, bv, _ := dst.At(nx, ny).RGBA()
				sumR += uint64(rv)
				sumG += uint64(gv)
				sumB += uint64(bv)
				count++
			}
		}
	}

	if count > 0 {
		strength := r.opts.Strength
		origR, origG, origB, origA := dst.At(x, y).RGBA()
		newR := uint32(float64(sumR/uint64(count))*(1-strength) + float64(origR)*strength)
		newG := uint32(float64(sumG/uint64(count))*(1-strength) + float64(origG)*strength)
		newB := uint32(float64(sumB/uint64(count))*(1-strength) + float64(origB)*strength)
		dst.Set(x, y, color.RGBA{
			R: clampUint8(int(newR) >> 8),
			G: clampUint8(int(newG) >> 8),
			B: clampUint8(int(newB) >> 8),
			A: clampUint8(int(origA) >> 8),
		})
	}
}

// repairStains 修复污渍
func (r *Restorer) repairStains(dst *image.RGBA, bounds image.Rectangle) {
	// 检测不规则形状的色斑并修复
	for y := bounds.Min.Y + 3; y < bounds.Max.Y-3; y++ {
		for x := bounds.Min.X + 3; x < bounds.Max.X-3; x++ {
			if r.detectStain(dst, x, y, bounds) {
				r.replaceWithNeighborhood(dst, x, y, bounds)
			}
		}
	}
}

// detectStain 检测污渍（颜色异常区域）
func (r *Restorer) detectStain(src *image.RGBA, x, y int, bounds image.Rectangle) bool {
	centerR, centerG, centerB, _ := src.At(x, y).RGBA()

	// 计算局部颜色统计
	var sumR, sumG, sumB uint64
	var sqSumR, sqSumG, sqSumB float64
	count := 0
	radius := 4

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			nx, ny := x+dx, y+dy
			if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
				rv, gv, bv, _ := src.At(nx, ny).RGBA()
				sumR += uint64(rv)
				sumG += uint64(gv)
				sumB += uint64(bv)
				sqSumR += float64(rv) * float64(rv)
				sqSumG += float64(gv) * float64(gv)
				sqSumB += float64(bv) * float64(bv)
				count++
			}
		}
	}

	if count < 5 {
		return false
	}

	// 计算局部方差
	n := float64(count)
	meanR := float64(sumR) / n
	meanG := float64(sumG) / n
	meanB := float64(sumB) / n

	varR := sqSumR/n - meanR*meanR
	varG := sqSumG/n - meanG*meanG
	varB := sqSumB/n - meanB*meanB

	// 检查当前像素是否偏离局部均值
	diffR := math.Abs(float64(centerR)-meanR) / math.Max(math.Sqrt(varR), 1)
	diffG := math.Abs(float64(centerG)-meanG) / math.Max(math.Sqrt(varG), 1)
	diffB := math.Abs(float64(centerB)-meanB) / math.Max(math.Sqrt(varB), 1)

	// 偏离超过阈值的异常像素视为污渍
	threshold := 2.0 + (1.0-r.opts.Strength)*2.0 // 修复强度越高，阈值越低
	return diffR > threshold || diffG > threshold || diffB > threshold
}

// enhanceFace 人脸增强（检测并增强面部区域）
func (r *Restorer) enhanceFace(dst *image.RGBA, bounds image.Rectangle) {
	// 简化实现：在图像中心区域（通常为人脸位置）应用局部对比度增强
	centerX := bounds.Min.X + bounds.Dx()/2
	centerY := bounds.Min.Y + bounds.Dy()/3 // 人脸通常在上1/3区域
	radius := minInt(bounds.Dx(), bounds.Dy()) / 4

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// 仅增强中心区域
			dx := float64(x - centerX)
			dy := float64(y - centerY)
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > float64(radius) {
				continue
			}

			// 距离权重（中心增强更多）
			weight := (1.0 - dist/float64(radius)) * 0.3 * r.opts.Strength

			rv, gv, bv, av := dst.At(x, y).RGBA()
			// 增强局部对比度
			newR := float64(rv) * (1 + weight)
			newG := float64(gv) * (1 + weight)
			newB := float64(bv) * (1 + weight)

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(newR) >> 8),
				G: clampUint8(int(newG) >> 8),
				B: clampUint8(int(newB) >> 8),
				A: clampUint8(int(av) >> 8),
			})
		}
	}
}

// restoreColorContrast 修复色彩和对比度
func (r *Restorer) restoreColorContrast(dst *image.RGBA, bounds image.Rectangle) {
	// 统计直方图
	var histR, histG, histB [256]int
	totalPixels := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rv, gv, bv, _ := dst.At(x, y).RGBA()
			histR[rv>>8]++
			histG[gv>>8]++
			histB[bv>>8]++
			totalPixels++
		}
	}

	if totalPixels == 0 {
		return
	}

	// 计算暗部和亮部截断点
	clipPercent := 0.005 * (1.0 - r.opts.Strength)
	lowClip := int(float64(totalPixels) * clipPercent)
	highClip := int(float64(totalPixels) * (1.0 - clipPercent))

	minR, maxR := findHistogramRange(histR[:], lowClip, highClip, totalPixels)
	minG, maxG := findHistogramRange(histG[:], lowClip, highClip, totalPixels)
	minB, maxB := findHistogramRange(histB[:], lowClip, highClip, totalPixels)

	// 应用自动色阶
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rv, gv, bv, av := dst.At(x, y).RGBA()
			newR := stretchHistogram(rv>>8, minR, maxR)
			newG := stretchHistogram(gv>>8, minG, maxG)
			newB := stretchHistogram(bv>>8, minB, maxB)

			// 混合原图和修复图
			strength := r.opts.Strength
			finalR := uint32(float64(rv>>8)*(1-strength) + float64(newR)*strength)
			finalG := uint32(float64(gv>>8)*(1-strength) + float64(newG)*strength)
			finalB := uint32(float64(bv>>8)*(1-strength) + float64(newB)*strength)

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(finalR)),
				G: clampUint8(int(finalG)),
				B: clampUint8(int(finalB)),
				A: clampUint8(int(av) >> 8),
			})
		}
	}
}

// findHistogramRange 查找直方图范围
func findHistogramRange(hist []int, lowClip, highClip, total int) (int, int) {
	minV, maxV := 0, 255
	cumSum := 0

	for i, count := range hist {
		cumSum += count
		if cumSum >= lowClip {
			minV = i
			break
		}
	}

	cumSum = 0
	for i, count := range hist {
		cumSum += count
		if cumSum >= highClip {
			maxV = i
			break
		}
	}

	if maxV <= minV {
		maxV = minV + 1
	}

	return minV, maxV
}

// stretchHistogram 拉伸直方图值
func stretchHistogram(value uint32, minV, maxV int) uint32 {
	if maxV == minV {
		return value
	}
	stretched := (float64(value) - float64(minV)) / float64(maxV-minV) * 255.0
	return uint32(clampUint8(int(stretched)))
}

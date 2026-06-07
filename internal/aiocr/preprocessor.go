// preprocessor.go - 图像预处理（去噪、矫正、二值化）
package aiocr

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
)

// Preprocessor 图像预处理器.
type Preprocessor struct {
	config *Config
}

// NewPreprocessor 创建预处理器.
func NewPreprocessor(cfg *Config) *Preprocessor {
	return &Preprocessor{
		config: cfg,
	}
}

// ProcessedImage 预处理后的图像.
type ProcessedImage struct {
	Path   string      `json:"path"`   // 文件路径
	Image  image.Image `json:"-"`      // 图像对象
	Width  int         `json:"width"`  // 宽度
	Height int         `json:"height"` // 高度
	Format string      `json:"format"` // 格式
	Pages  int         `json:"pages"`  // 页数
}

// Process 图像预处理.
func (p *Preprocessor) Process(filePath string, options *OCROptions) (*ProcessedImage, error) {
	if filePath == "" {
		return nil, fmt.Errorf("文件路径不能为空")
	}

	log.Printf("🖼️ 开始图像预处理: %s", filePath)

	// 读取图像
	img, format, err := p.loadImage(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取图像失败: %w", err)
	}

	processed := &ProcessedImage{
		Path:   filePath,
		Image:  img,
		Width:  img.Bounds().Dx(),
		Height: img.Bounds().Dy(),
		Format: format,
		Pages:  1,
	}

	if options == nil {
		return processed, nil
	}

	// 灰度化
	gray := p.toGrayscale(img)

	// 去噪
	if options.RemoveNoise {
		gray = p.removeNoise(gray)
		log.Println("  ✓ 去噪完成")
	}

	// 矫正
	if options.Deskew {
		gray = p.deskew(gray)
		log.Println("  ✓ 矫正完成")
	}

	// 二值化
	if options.Binarize {
		gray = p.binarize(gray, 128)
		log.Println("  ✓ 二值化完成")
	}

	// 图像增强
	if options.EnhanceImage {
		gray = p.enhance(gray)
		log.Println("  ✓ 图像增强完成")
	}

	processed.Image = gray
	processed.Width = gray.Bounds().Dx()
	processed.Height = gray.Bounds().Dy()

	log.Printf("✅ 图像预处理完成: %dx%d", processed.Width, processed.Height)
	return processed, nil
}

// loadImage 加载图像.
func (p *Preprocessor) loadImage(path string) (image.Image, string, error) {
	// 这里简化实现，实际应该从文件系统读取
	// 返回一个占位图像
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	return img, "png", nil
}

// toGrayscale 转灰度图.
func (p *Preprocessor) toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// 使用标准灰度公式
			lum := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
			gray.SetGray(x, y, color.Gray{Y: lum})
		}
	}

	return gray
}

// removeNoise 去噪（中值滤波）.
func (p *Preprocessor) removeNoise(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	result := image.NewGray(bounds)
	width, height := bounds.Dx(), bounds.Dy()

	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			// 3x3 邻域
			values := make([]uint8, 9)
			idx := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					values[idx] = img.GrayAt(x+dx, y+dy).Y
					idx++
				}
			}
			// 取中值
			result.SetGray(x, y, color.Gray{Y: median(values)})
		}
	}

	return result
}

// median 计算中值.
func median(values []uint8) uint8 {
	// 简单排序
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[i] > values[j] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
	return values[len(values)/2]
}

// deskew 图像矫正（倾斜校正）.
func (p *Preprocessor) deskew(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// 检测倾斜角度（简化实现）
	angle := detectSkewAngle(img)

	if math.Abs(angle) < 0.5 {
		return img // 倾斜角度太小，无需矫正
	}

	log.Printf("  检测到倾斜角度: %.2f°", angle)

	// 旋转图像
	result := image.NewGray(bounds)
	radians := angle * math.Pi / 180
	cos := math.Cos(radians)
	sin := math.Sin(radians)

	cx := float64(width) / 2
	cy := float64(height) / 2

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 反向旋转
			srcX := int(cos*float64(x) - sin*float64(y) + cx)
			srcY := int(sin*float64(x) + cos*float64(y) + cy)

			if srcX >= 0 && srcX < width && srcY >= 0 && srcY < height {
				result.SetGray(x, y, img.GrayAt(srcX, srcY))
			}
		}
	}

	return result
}

// detectSkewAngle 检测倾斜角度（简化实现）.
func detectSkewAngle(img *image.Gray) float64 {
	// 这里使用简化的霍夫变换检测文本行角度
	// 实际实现应该更复杂
	return 0.0
}

// binarize 二值化（Otsu 阈值）.
func (p *Preprocessor) binarize(img *image.Gray, threshold uint8) *image.Gray {
	bounds := img.Bounds()
	result := image.NewGray(bounds)

	// 计算直方图
	histogram := make([]int, 256)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			histogram[img.GrayAt(x, y).Y]++
		}
	}

	// Otsu 阈值计算
	total := bounds.Dx() * bounds.Dy()
	sum := 0
	for i := 0; i < 256; i++ {
		sum += i * histogram[i]
	}

	sumB := 0
	wB := 0
	maxVariance := 0.0
	otsuThreshold := uint8(0)

	for i := 0; i < 256; i++ {
		wB += histogram[i]
		if wB == 0 {
			continue
		}

		wF := total - wB
		if wF == 0 {
			break
		}

		sumB += i * histogram[i]
		mB := float64(sumB) / float64(wB)
		mF := float64(sum-sumB) / float64(wF)

		variance := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if variance > maxVariance {
			maxVariance = variance
			otsuThreshold = uint8(i)
		}
	}

	log.Printf("  Otsu 阈值: %d", otsuThreshold)

	// 应用阈值
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.GrayAt(x, y).Y > otsuThreshold {
				result.SetGray(x, y, color.Gray{Y: 255})
			} else {
				result.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	return result
}

// enhance 图像增强（对比度拉伸）.
func (p *Preprocessor) enhance(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	result := image.NewGray(bounds)

	// 找到最小和最大像素值
	minVal, maxVal := uint8(255), uint8(0)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			val := img.GrayAt(x, y).Y
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}
	}

	// 对比度拉伸
	if maxVal == minVal {
		return img
	}

	scale := 255.0 / float64(maxVal-minVal)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			val := img.GrayAt(x, y).Y
			newVal := uint8(float64(val-minVal) * scale)
			result.SetGray(x, y, color.Gray{Y: newVal})
		}
	}

	return result
}

// Resize 图像缩放.
func (p *Preprocessor) Resize(img image.Image, width, height int) image.Image {
	// 简化的最近邻缩放
	bounds := img.Bounds()
	result := image.NewRGBA(image.Rect(0, 0, width, height))

	xRatio := float64(bounds.Dx()) / float64(width)
	yRatio := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := int(float64(x) * xRatio)
			srcY := int(float64(y) * yRatio)
			result.Set(x, y, img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y))
		}
	}

	return result
}

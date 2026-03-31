// Package ai - Scene recognition implementation
package ai

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
)

// SceneRecognizer provides scene recognition for photos
type SceneRecognizer struct {
	config       *ModelConfig
	initialized  bool
	mu           sync.RWMutex

	// Scene categories with keywords
	sceneKeywords map[string][]string

	// Color mood mapping
	moodColors map[string][]color.RGBA
}

// NewSceneRecognizer creates a new scene recognizer
func NewSceneRecognizer(config *ModelConfig) (*SceneRecognizer, error) {
	if config == nil {
		config = DefaultModelConfig()
	}

	sr := &SceneRecognizer{
		config: config,
	}

	sr.initSceneKeywords()
	sr.initMoodColors()
	sr.initialized = true

	return sr, nil
}

// initSceneKeywords initializes scene keyword mappings
func (sr *SceneRecognizer) initSceneKeywords() {
	sr.sceneKeywords = map[string][]string{
		"beach": {"sand", "ocean", "sea", "waves", "beach", "shore", "coastline", "tropical"},
		"mountain": {"mountain", "peak", "summit", "ridge", "alpine", "snow", "cliff"},
		"city": {"building", "skyscraper", "street", "urban", "downtown", "city", "architecture"},
		"forest": {"forest", "trees", "woods", "jungle", "greenery", "wilderness"},
		"desert": {"desert", "sand", "dunes", "arid", "canyon"},
		"lake": {"lake", "pond", "water", "reflection", "calm"},
		"river": {"river", "stream", "creek", "waterfall", "rapids"},
		"snow": {"snow", "winter", "ice", "frost", "cold"},
		"flower": {"flower", "bloom", "garden", "petals", "blossom"},
		"sunset": {"sunset", "dusk", "twilight", "golden hour", "evening"},
		"sunrise": {"sunrise", "dawn", "morning", "first light"},
		"night": {"night", "stars", "moonlight", "dark", "evening"},
		"indoor": {"interior", "room", "home", "furniture", "indoor"},
		"food": {"food", "dish", "cuisine", "meal", "cooking", "restaurant"},
		"portrait": {"face", "person", "portrait", "selfie"},
		"pet": {"dog", "cat", "pet", "animal", "puppy", "kitten"},
		"wedding": {"wedding", "bride", "groom", "ceremony", "marriage"},
		"sports": {"sports", "game", "athletic", "stadium", "fitness"},
		"concert": {"concert", "music", "stage", "performance", "band"},
		"travel": {"landmark", "monument", "tourism", "sightseeing", "vacation"},
	}
}

// initMoodColors initializes mood color mappings
func (sr *SceneRecognizer) initMoodColors() {
	sr.moodColors = map[string][]color.RGBA{
		"happy": {
			{R: 255, G: 215, B: 0, A: 255},   // Gold
			{R: 255, G: 182, B: 193, A: 255}, // Light pink
			{R: 144, G: 238, B: 144, A: 255}, // Light green
		},
		"calm": {
			{R: 135, G: 206, B: 235, A: 255}, // Sky blue
			{R: 176, G: 224, B: 230, A: 255}, // Powder blue
			{R: 152, G: 251, B: 152, A: 255}, // Pale green
		},
		"energetic": {
			{R: 255, G: 69, B: 0, A: 255},    // Orange red
			{R: 255, G: 165, B: 0, A: 255},   // Orange
			{R: 255, G: 255, B: 0, A: 255},   // Yellow
		},
		"melancholic": {
			{R: 70, G: 130, B: 180, A: 255},  // Steel blue
			{R: 128, G: 128, B: 128, A: 255}, // Gray
			{R: 72, G: 61, B: 139, A: 255},   // Dark slate blue
		},
	}
}

// RecognizeScene analyzes a photo and returns scene classification
func (sr *SceneRecognizer) RecognizeScene(ctx context.Context, img image.Image) (*SceneRecognitionResult, error) {
	result := &SceneRecognitionResult{
		PhotoID:    "",
		Categories: make([]SceneInfo, 0),
		Objects:    make([]ObjectInfo, 0),
		Colors:     make([]ColorInfo, 0),
	}

	// 1. Extract colors
	colors := sr.extractColors(img, 5)
	result.Colors = colors

	// 2. Analyze brightness and contrast
	brightness, contrast := sr.analyzeBrightnessContrast(img)

	// 3. Detect time of day
	result.TimeOfDay = sr.detectTimeOfDay(brightness, colors)

	// 4. Detect season
	result.Season = sr.detectSeason(colors, brightness)

	// 5. Analyze composition
	composition := sr.analyzeComposition(img)

	// 6. Classify scene
	sceneInfo := sr.classifyScene(img, colors, brightness, contrast, composition)
	result.Primary = sceneInfo
	result.Categories = append(result.Categories, sceneInfo)

	// 7. Detect mood
	result.Mood = sr.detectMood(colors, brightness, contrast)

	// 8. Detect objects (simplified)
	objects := sr.detectObjects(img)
	result.Objects = objects

	return result, nil
}

// extractColors extracts dominant colors from image
func (sr *SceneRecognizer) extractColors(img image.Image, numColors int) []ColorInfo {
	bounds := img.Bounds()
	colorMap := make(map[color.RGBA]int)

	// Sample pixels
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			// Quantize to reduce color count
			qr, qg, qb := uint8(r>>8), uint8(g>>8), uint8(b>>8)
			// Reduce precision
			qr = (qr / 16) * 16
			qg = (qg / 16) * 16
			qb = (qb / 16) * 16
			c := color.RGBA{R: qr, G: qg, B: qb, A: 255}
			colorMap[c]++
		}
	}

	// Sort by frequency
	type colorCount struct {
		color color.RGBA
		count int
	}
	colors := make([]colorCount, 0, len(colorMap))
	for c, count := range colorMap {
		colors = append(colors, colorCount{c, count})
	}

	sort.Slice(colors, func(i, j int) bool {
		return colors[i].count > colors[j].count
	})

	// Get top colors
	totalPixels := (bounds.Dx() / 4) * (bounds.Dy() / 4)
	result := make([]ColorInfo, 0, numColors)
	for i := 0; i < numColors && i < len(colors); i++ {
		percent := float64(colors[i].count) / float64(totalPixels) * 100
		result = append(result, ColorInfo{
			Hex:     fmt.Sprintf("#%02X%02X%02X", colors[i].color.R, colors[i].color.G, colors[i].color.B),
			Name:    sr.colorName(colors[i].color),
			Percent: percent,
		})
	}

	return result
}

// colorName returns a human-readable color name
func (sr *SceneRecognizer) colorName(c color.RGBA) string {
	// Convert to HSL for easier naming
	r, g, b := float64(c.R)/255.0, float64(c.G)/255.0, float64(c.B)/255.0
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l := (max + min) / 2

	// Lightness-based names
	if l > 0.9 {
		return "white"
	}
	if l < 0.1 {
		return "black"
	}

	// Calculate hue
	var h, s float64
	if max != min {
		d := max - min
		s = d / (1 - math.Abs(2*l-1))
		switch max {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = 2 + (b-r)/d
		case b:
			h = 4 + (r-g)/d
		}
		h /= 6
	}

	// Saturation-based names
	if s < 0.1 {
		if l < 0.3 {
			return "dark gray"
		}
		if l > 0.7 {
			return "light gray"
		}
		return "gray"
	}

	// Hue-based names
	switch {
	case h < 0.05 || h >= 0.95:
		return "red"
	case h < 0.1:
		return "orange"
	case h < 0.15:
		return "yellow"
	case h < 0.35:
		return "green"
	case h < 0.5:
		return "cyan"
	case h < 0.65:
		return "blue"
	case h < 0.8:
		return "purple"
	default:
		return "pink"
	}
}

// analyzeBrightnessContrast analyzes image brightness and contrast
func (sr *SceneRecognizer) analyzeBrightnessContrast(img image.Image) (float64, float64) {
	bounds := img.Bounds()
	var sum, sumSq float64
	count := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to luminance
			lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
			sum += lum
			sumSq += lum * lum
			count++
		}
	}

	if count == 0 {
		return 0, 0
	}

	brightness := sum / float64(count) / 65535.0
	meanSq := sumSq / float64(count)
	variance := meanSq - (sum/float64(count))*(sum/float64(count))
	contrast := math.Sqrt(variance) / 65535.0

	return brightness, contrast
}

// detectTimeOfDay estimates time of day from image characteristics
func (sr *SceneRecognizer) detectTimeOfDay(brightness float64, colors []ColorInfo) string {
	// Check for golden hour colors (warm oranges and yellows)
	warmColors := 0.0
	for _, c := range colors {
		if c.Name == "orange" || c.Name == "yellow" || c.Name == "red" {
			warmColors += c.Percent
		}
	}

	// Night detection
	if brightness < 0.2 {
		return "night"
	}

	// Sunset/sunrise detection
	if warmColors > 20 && brightness < 0.6 {
		return "evening"
	}
	if warmColors > 20 && brightness > 0.6 {
		return "morning"
	}

	// Daytime
	if brightness > 0.7 {
		return "afternoon"
	}

	return "morning"
}

// detectSeason estimates season from colors
func (sr *SceneRecognizer) detectSeason(colors []ColorInfo, brightness float64) string {
	// Check for seasonal colors
	greenPercent := 0.0
	yellowPercent := 0.0
	whitePercent := 0.0
	bluePercent := 0.0

	for _, c := range colors {
		switch c.Name {
		case "green":
			greenPercent += c.Percent
		case "yellow", "orange":
			yellowPercent += c.Percent
		case "white":
			whitePercent += c.Percent
		case "blue":
			bluePercent += c.Percent
		}
	}

	// Winter: white (snow) or low green
	if whitePercent > 15 {
		return "winter"
	}

	// Spring: fresh green
	if greenPercent > 30 && brightness > 0.5 {
		return "spring"
	}

	// Summer: bright, lots of blue (sky)
	if bluePercent > 20 && brightness > 0.6 {
		return "summer"
	}

	// Autumn: yellow/orange tones
	if yellowPercent > 20 {
		return "autumn"
	}

	return "summer" // Default
}

// analyzeComposition analyzes image composition
func (sr *SceneRecognizer) analyzeComposition(img image.Image) map[string]float64 {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Rule of thirds regions
	regions := [9]float64{}
	regionCounts := [9]int{}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)

			// Determine region
			var row, col int
			if y < height/3 {
				row = 0
			} else if y < height*2/3 {
				row = 1
			} else {
				row = 2
			}
			if x < width/3 {
				col = 0
			} else if x < width*2/3 {
				col = 1
			} else {
				col = 2
			}
			idx := row*3 + col
			regions[idx] += lum
			regionCounts[idx]++
		}
	}

	// Normalize
	for i := range regions {
		if regionCounts[i] > 0 {
			regions[i] /= float64(regionCounts[i])
		}
	}

	// Calculate composition metrics
	centerRegion := (regions[4] + regions[3] + regions[5]) / 3
	cornerRegions := (regions[0] + regions[2] + regions[6] + regions[8]) / 4

	return map[string]float64{
		"center_weight": centerRegion / 65535.0,
		"corner_weight": cornerRegions / 65535.0,
		"balance":       math.Abs(centerRegion - cornerRegions) / 65535.0,
	}
}

// classifyScene classifies the main scene
func (sr *SceneRecognizer) classifyScene(img image.Image, colors []ColorInfo, brightness, contrast float64, composition map[string]float64) SceneInfo {
	bounds := img.Bounds()
	aspectRatio := float64(bounds.Dx()) / float64(bounds.Dy())

	// Analyze colors
	bluePercent := 0.0
	greenPercent := 0.0
	yellowPercent := 0.0
	whitePercent := 0.0
	skinPercent := 0.0

	for _, c := range colors {
		switch c.Name {
		case "blue":
			bluePercent += c.Percent
		case "green":
			greenPercent += c.Percent
		case "yellow", "orange":
			yellowPercent += c.Percent
		case "white":
			whitePercent += c.Percent
		}
	}

	// Detect skin tones for portrait detection
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 8 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := r>>8, g>>8, b>>8

			// Simple skin tone detection
			if r8 > 95 && g8 > 40 && b8 > 20 &&
				r8 > g8 && r8 > b8 &&
				math.Abs(float64(r8)-float64(g8)) > 15 {
				skinPercent++
			}
		}
	}
	totalSamples := float64(bounds.Dx()/8) * float64(bounds.Dy()/8)
	if totalSamples > 0 {
		skinPercent = skinPercent / totalSamples * 100
	}

	// Scene classification rules
	scene := "other"
	confidence := 0.5
	var labels []string

	// Portrait detection
	if skinPercent > 30 {
		scene = "portrait"
		confidence = 0.8 + skinPercent/500
		labels = []string{"people", "faces"}
		return SceneInfo{Category: scene, Confidence: math.Min(confidence, 1.0), Labels: labels}
	}

	// Beach detection
	if bluePercent > 25 && (yellowPercent > 10 || whitePercent > 10) {
		scene = "beach"
		confidence = 0.75
		labels = []string{"ocean", "sand", "vacation"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// Mountain/Snow detection
	if whitePercent > 20 {
		scene = "snow"
		confidence = 0.8
		labels = []string{"winter", "mountain", "cold"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// Forest/Nature detection
	if greenPercent > 35 {
		scene = "forest"
		confidence = 0.75
		labels = []string{"nature", "trees", "outdoor"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// City detection
	if contrast > 0.3 && composition["center_weight"] > 0.5 {
		scene = "city"
		confidence = 0.7
		labels = []string{"urban", "architecture", "street"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// Night detection
	if brightness < 0.25 {
		scene = "night"
		confidence = 0.75
		labels = []string{"dark", "evening", "nocturnal"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// Sunset/Sunrise detection
	if yellowPercent > 25 && brightness > 0.4 && brightness < 0.7 {
		scene = "sunset"
		confidence = 0.8
		labels = []string{"golden hour", "evening", "warm"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// Landscape detection (wide aspect ratio)
	if aspectRatio > 1.5 && bluePercent > 20 {
		scene = "landscape"
		confidence = 0.7
		labels = []string{"scenic", "outdoor", "nature"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// Indoor detection
	if brightness > 0.3 && brightness < 0.6 && greenPercent < 10 && bluePercent < 15 {
		scene = "indoor"
		confidence = 0.65
		labels = []string{"interior", "home", "room"}
		return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
	}

	// Default
	labels = []string{"general"}
	return SceneInfo{Category: scene, Confidence: confidence, Labels: labels}
}

// detectMood detects image mood
func (sr *SceneRecognizer) detectMood(colors []ColorInfo, brightness, contrast float64) string {
	// Calculate color mood scores
	moodScores := map[string]float64{
		"happy":       0,
		"calm":        0,
		"energetic":   0,
		"melancholic": 0,
	}

	for _, c := range colors {
		switch c.Name {
		case "yellow", "orange", "pink":
			moodScores["happy"] += c.Percent
			moodScores["energetic"] += c.Percent * 0.5
		case "blue", "cyan":
			moodScores["calm"] += c.Percent
			if brightness < 0.4 {
				moodScores["melancholic"] += c.Percent * 0.5
			}
		case "green":
			moodScores["calm"] += c.Percent * 0.7
		case "purple":
			moodScores["melancholic"] += c.Percent * 0.5
		case "red":
			moodScores["energetic"] += c.Percent
		}
	}

	// Adjust by brightness
	if brightness > 0.7 {
		moodScores["happy"] += 10
		moodScores["energetic"] += 5
	} else if brightness < 0.3 {
		moodScores["melancholic"] += 10
		moodScores["calm"] += 5
	}

	// Find max mood
	maxMood := "calm"
	maxScore := moodScores["calm"]
	for mood, score := range moodScores {
		if score > maxScore {
			maxScore = score
			maxMood = mood
		}
	}

	return maxMood
}

// detectObjects detects objects in image (simplified)
func (sr *SceneRecognizer) detectObjects(img image.Image) []ObjectInfo {
	// This is a simplified implementation
	// In production, use YOLO or similar object detection model
	objects := make([]ObjectInfo, 0)

	bounds := img.Bounds()

	// Simple edge detection for object presence
	edgeCount := 0
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y += 4 {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x += 4 {
			r1, g1, b1, _ := img.At(x-1, y).RGBA()
			r2, g2, b2, _ := img.At(x+1, y).RGBA()
			r3, g3, b3, _ := img.At(x, y-1).RGBA()
			r4, g4, b4, _ := img.At(x, y+1).RGBA()

			gx := math.Abs(float64(r2)-float64(r1)) + math.Abs(float64(g2)-float64(g1)) + math.Abs(float64(b2)-float64(b1))
			gy := math.Abs(float64(r4)-float64(r3)) + math.Abs(float64(g4)-float64(g3)) + math.Abs(float64(b4)-float64(b3))

			if math.Sqrt(gx*gx+gy*gy) > 30000 {
				edgeCount++
			}
		}
	}

	// High edge count suggests objects present
	if edgeCount > 100 {
		objects = append(objects, ObjectInfo{
			Label:      "objects",
			Confidence: math.Min(float64(edgeCount)/500.0, 1.0),
		})
	}

	return objects
}

// BatchRecognize recognizes scenes for multiple images
func (sr *SceneRecognizer) BatchRecognize(ctx context.Context, images []image.Image) ([]*SceneRecognitionResult, error) {
	results := make([]*SceneRecognitionResult, len(images))

	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, len(images))

	for i, img := range images {
		wg.Add(1)
		go func(idx int, image image.Image) {
			defer wg.Done()
			result, err := sr.RecognizeScene(ctx, image)
			mu.Lock()
			results[idx] = result
			errors[idx] = err
			mu.Unlock()
		}(i, img)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("batch recognition had errors: %v", errors)
		}
	}

	return results, nil
}

// Close releases resources
func (sr *SceneRecognizer) Close() error {
	return nil
}

// Python scene classifier bridge (for advanced models)
type PythonSceneClassifier struct {
	pythonPath string
	scriptPath string
}

func NewPythonSceneClassifier(modelPath string) (*PythonSceneClassifier, error) {
	pythonPath := "python3"
	if path, err := exec.LookPath("python3"); err == nil {
		pythonPath = path
	}

	scriptPath := filepath.Join(os.TempDir(), "scene_classifier.py")
	script := generateSceneScript(modelPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("failed to write scene script: %w", err)
	}

	return &PythonSceneClassifier{
		pythonPath: pythonPath,
		scriptPath: scriptPath,
	}, nil
}

func generateSceneScript(modelPath string) string {
	return `#!/usr/bin/env python3
"""Scene Classification Bridge"""
import sys
import json
import base64
import io

try:
    import torch
    from PIL import Image
    from torchvision import transforms, models
except ImportError as e:
    print(json.dumps({"error": str(e)}), flush=True)
    sys.exit(1)

# Load model
device = "cuda" if torch.cuda.is_available() else "cpu"
model = models.resnet50(pretrained=True)
model = model.to(device)
model.eval()

# ImageNet labels (simplified scene mapping)
SCENE_LABELS = {
    "beach": ["seashore", "lakeside", "sandbar"],
    "mountain": ["alp", "mountain", "valley"],
    "city": ["street", "building", "skyscraper"],
    "forest": ["forest", "woods", "jungle"],
    "indoor": ["room", "home", "interior"],
}

preprocess = transforms.Compose([
    transforms.Resize(256),
    transforms.CenterCrop(224),
    transforms.ToTensor(),
    transforms.Normalize(mean=[0.485, 0.456, 0.406], std=[0.229, 0.224, 0.225]),
])

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue

    try:
        cmd = json.loads(line)
        if cmd.get("action") == "classify":
            img_data = base64.b64decode(cmd["image"])
            img = Image.open(io.BytesIO(img_data)).convert("RGB")
            input_tensor = preprocess(img).unsqueeze(0).to(device)

            with torch.no_grad():
                output = model(input_tensor)
                probs = torch.nn.functional.softmax(output[0], dim=0)
                top5 = torch.topk(probs, 5)

                result = {"categories": []}
                for i in range(5):
                    result["categories"].append({
                        "idx": top5.indices[i].item(),
                        "confidence": top5.values[i].item(),
                    })

                print(json.dumps(result), flush=True)
    except Exception as e:
        print(json.dumps({"error": str(e)}), flush=True)
`
}
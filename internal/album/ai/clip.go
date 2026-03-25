// Package ai - CLIP model implementation for semantic search
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// CLIPModel provides CLIP-based semantic search capabilities
// CLIP (Contrastive Language-Image Pre-training) enables searching images with text
type CLIPModel struct {
	config      *ModelConfig
	modelPath   string
	initialized bool
	mu          sync.RWMutex

	// ONNX session (if using ONNX Runtime)
	onnxSession interface{}

	// Python bridge for models without ONNX support
	pythonBridge *PythonBridge
}

// PythonBridge bridges to Python CLIP implementation
type PythonBridge struct {
	pythonPath string
	scriptPath string
	process    *exec.Cmd
	stdin      *os.File
	stdout     *os.File
}

// NewCLIPModel creates a new CLIP model instance
func NewCLIPModel(config *ModelConfig) (*CLIPModel, error) {
	if config == nil {
		config = DefaultModelConfig()
	}

	clip := &CLIPModel{
		config:    config,
		modelPath: config.CLIPModelPath,
	}

	// Initialize model
	if err := clip.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize CLIP model: %w", err)
	}

	return clip, nil
}

// initialize loads the CLIP model
func (c *CLIPModel) initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Check if ONNX model exists
	if c.config.UseONNXRuntime && c.modelPath != "" {
		if _, err := os.Stat(c.modelPath); err == nil {
			// Load ONNX model
			if err := c.loadONNXModel(); err != nil {
				// Fall back to Python bridge
				fmt.Printf("Warning: ONNX model load failed, using Python bridge: %v\n", err)
				return c.initPythonBridge()
			}
			c.initialized = true
			return nil
		}
	}

	// Use Python bridge for CLIP
	if err := c.initPythonBridge(); err != nil {
		return fmt.Errorf("failed to init Python bridge: %w", err)
	}

	c.initialized = true
	return nil
}

// loadONNXModel loads ONNX model (placeholder for actual implementation)
func (c *CLIPModel) loadONNXModel() error {
	// TODO: Implement ONNX Runtime loading
	// This requires github.com/yalue/onnxruntime_go or similar
	return fmt.Errorf("ONNX Runtime not implemented yet")
}

// initPythonBridge initializes Python bridge for CLIP
func (c *CLIPModel) initPythonBridge() error {
	// Find Python executable
	pythonPath := "python3"
	if path, err := exec.LookPath("python3"); err == nil {
		pythonPath = path
	} else if path, err := exec.LookPath("python"); err == nil {
		pythonPath = path
	}

	// Create Python script for CLIP inference
	scriptPath := filepath.Join(os.TempDir(), "clip_bridge.py")
	script := c.generateCLIPScript()
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write CLIP bridge script: %w", err)
	}

	c.pythonBridge = &PythonBridge{
		pythonPath: pythonPath,
		scriptPath: scriptPath,
	}

	return nil
}

// generateCLIPScript generates Python script for CLIP inference
func (c *CLIPModel) generateCLIPScript() string {
	return `#!/usr/bin/env python3
"""CLIP Bridge for semantic image search"""
import sys
import json
import base64
import io
import numpy as np

try:
    import torch
    from PIL import Image
    import clip
except ImportError as e:
    print(json.dumps({"error": str(e)}), flush=True)
    sys.exit(1)

# Load CLIP model
device = "cuda" if torch.cuda.is_available() else "cpu"
model_name = "` + c.config.CLIPModelType + `"

try:
    model, preprocess = clip.load(model_name, device=device)
except:
    # Fall back to default model
    model, preprocess = clip.load("ViT-B/32", device=device)

def encode_text(text):
    """Encode text to embedding vector"""
    with torch.no_grad():
        tokens = clip.tokenize([text], truncate=True).to(device)
        embedding = model.encode_text(tokens)
        embedding = embedding / embedding.norm(dim=-1, keepdim=True)
        return embedding.cpu().numpy()[0].tolist()

def encode_image(image_data):
    """Encode image to embedding vector"""
    with torch.no_grad():
        # Decode base64 image
        if isinstance(image_data, str):
            img_bytes = base64.b64decode(image_data)
            img = Image.open(io.BytesIO(img_bytes))
        else:
            img = image_data

        # Preprocess and encode
        image_input = preprocess(img).unsqueeze(0).to(device)
        embedding = model.encode_image(image_input)
        embedding = embedding / embedding.norm(dim=-1, keepdim=True)
        return embedding.cpu().numpy()[0].tolist()

def compute_similarity(vec1, vec2):
    """Compute cosine similarity between two vectors"""
    v1 = np.array(vec1)
    v2 = np.array(vec2)
    return float(np.dot(v1, v2) / (np.linalg.norm(v1) * np.linalg.norm(v2)))

# Main loop - read JSON commands from stdin
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue

    try:
        cmd = json.loads(line)
        action = cmd.get("action")

        if action == "encode_text":
            result = {"embedding": encode_text(cmd["text"])}

        elif action == "encode_image":
            result = {"embedding": encode_image(cmd["image"])}

        elif action == "encode_images":
            embeddings = []
            for img_data in cmd["images"]:
                embeddings.append(encode_image(img_data))
            result = {"embeddings": embeddings}

        elif action == "similarity":
            result = {"similarity": compute_similarity(cmd["vec1"], cmd["vec2"])}

        else:
            result = {"error": f"Unknown action: {action}"}

        print(json.dumps(result), flush=True)

    except Exception as e:
        print(json.dumps({"error": str(e)}), flush=True)
`
}

// EncodeText encodes text to CLIP embedding
func (c *CLIPModel) EncodeText(ctx context.Context, text string) ([]float32, error) {
	if !c.initialized {
		if err := c.initialize(); err != nil {
			return nil, err
		}
	}

	// Use Python bridge
	result, err := c.callPythonBridge(map[string]interface{}{
		"action": "encode_text",
		"text":   text,
	})
	if err != nil {
		return nil, err
	}

	if emb, ok := result["embedding"].([]interface{}); ok {
		vector := make([]float32, len(emb))
		for i, v := range emb {
			if f, ok := v.(float64); ok {
				vector[i] = float32(f)
			}
		}
		return vector, nil
	}

	return nil, fmt.Errorf("invalid response from CLIP model")
}

// EncodeImage encodes image to CLIP embedding
func (c *CLIPModel) EncodeImage(ctx context.Context, img image.Image) ([]float32, error) {
	if !c.initialized {
		if err := c.initialize(); err != nil {
			return nil, err
		}
	}

	// Convert image to base64
	imgData, err := imageToBase64(img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	result, err := c.callPythonBridge(map[string]interface{}{
		"action": "encode_image",
		"image":  imgData,
	})
	if err != nil {
		return nil, err
	}

	if emb, ok := result["embedding"].([]interface{}); ok {
		vector := make([]float32, len(emb))
		for i, v := range emb {
			if f, ok := v.(float64); ok {
				vector[i] = float32(f)
			}
		}
		return vector, nil
	}

	return nil, fmt.Errorf("invalid response from CLIP model")
}

// EncodeImageBatch encodes multiple images in batch
func (c *CLIPModel) EncodeImageBatch(ctx context.Context, images []image.Image) ([][]float32, error) {
	if !c.initialized {
		if err := c.initialize(); err != nil {
			return nil, err
		}
	}

	// Convert images to base64
	imgDataList := make([]string, len(images))
	for i, img := range images {
		data, err := imageToBase64(img)
		if err != nil {
			return nil, fmt.Errorf("failed to encode image %d: %w", i, err)
		}
		imgDataList[i] = data
	}

	result, err := c.callPythonBridge(map[string]interface{}{
		"action": "encode_images",
		"images": imgDataList,
	})
	if err != nil {
		return nil, err
	}

	if embList, ok := result["embeddings"].([]interface{}); ok {
		vectors := make([][]float32, len(embList))
		for i, emb := range embList {
			if embArray, ok := emb.([]interface{}); ok {
				vector := make([]float32, len(embArray))
				for j, v := range embArray {
					if f, ok := v.(float64); ok {
						vector[j] = float32(f)
					}
				}
				vectors[i] = vector
			}
		}
		return vectors, nil
	}

	return nil, fmt.Errorf("invalid response from CLIP model")
}

// ComputeSimilarity computes cosine similarity between two vectors
func (c *CLIPModel) ComputeSimilarity(vec1, vec2 []float32) (float64, error) {
	result, err := c.callPythonBridge(map[string]interface{}{
		"action": "similarity",
		"vec1":   vec1,
		"vec2":   vec2,
	})
	if err != nil {
		return 0, err
	}

	if sim, ok := result["similarity"].(float64); ok {
		return sim, nil
	}

	return 0, fmt.Errorf("invalid similarity response")
}

// callPythonBridge sends a command to Python bridge and returns result
func (c *CLIPModel) callPythonBridge(cmd map[string]interface{}) (map[string]interface{}, error) {
	if c.pythonBridge == nil {
		return nil, fmt.Errorf("Python bridge not initialized")
	}

	// Start process if not running
	if c.pythonBridge.process == nil {
		cmd := exec.Command(c.pythonBridge.pythonPath, c.pythonBridge.scriptPath)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start Python process: %w", err)
		}

		c.pythonBridge.process = cmd
		c.pythonBridge.stdin = stdin.(*os.File)
		c.pythonBridge.stdout = stdout.(*os.File)
	}

	// Send command
	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	c.pythonBridge.stdin.Write(cmdJSON)
	c.pythonBridge.stdin.Write([]byte("\n"))

	// Read response
	var result map[string]interface{}
	decoder := json.NewDecoder(c.pythonBridge.stdout)
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if errMsg, ok := result["error"].(string); ok {
		return nil, fmt.Errorf("CLIP error: %s", errMsg)
	}

	return result, nil
}

// Close releases model resources
func (c *CLIPModel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pythonBridge != nil && c.pythonBridge.process != nil {
		c.pythonBridge.stdin.Close()
		c.pythonBridge.stdout.Close()
		if err := c.pythonBridge.process.Kill(); err != nil {
			return err
		}
	}

	c.initialized = false
	return nil
}

// SemanticSearch performs semantic search using CLIP embeddings
type SemanticSearch struct {
	clip    *CLIPModel
	index   VectorIndex
	photos  map[string]*PhotoVector
	mu      sync.RWMutex
}

// NewSemanticSearch creates a new semantic search instance
func NewSemanticSearch(clip *CLIPModel, index VectorIndex) (*SemanticSearch, error) {
	return &SemanticSearch{
		clip:   clip,
		index:  index,
		photos: make(map[string]*PhotoVector),
	}, nil
}

// IndexPhoto indexes a photo for semantic search
func (s *SemanticSearch) IndexPhoto(ctx context.Context, photoID string, img image.Image) error {
	// Get image embedding
	embedding, err := s.clip.EncodeImage(ctx, img)
	if err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	// Store in memory
	s.mu.Lock()
	s.photos[photoID] = &PhotoVector{
		PhotoID:     photoID,
		ImageVector: embedding,
		CreatedAt:   timeNow(),
		UpdatedAt:   timeNow(),
	}
	s.mu.Unlock()

	// Add to vector index
	if s.index != nil {
		if err := s.index.Add(photoID, embedding); err != nil {
			return fmt.Errorf("failed to add to index: %w", err)
		}
	}

	return nil
}

// IndexPhotoBatch indexes multiple photos
func (s *SemanticSearch) IndexPhotoBatch(ctx context.Context, photoIDs []string, images []image.Image) error {
	if len(photoIDs) != len(images) {
		return fmt.Errorf("photoIDs and images length mismatch")
	}

	// Get embeddings in batch
	embeddings, err := s.clip.EncodeImageBatch(ctx, images)
	if err != nil {
		return fmt.Errorf("failed to encode images: %w", err)
	}

	// Store in memory
	s.mu.Lock()
	for i, photoID := range photoIDs {
		s.photos[photoID] = &PhotoVector{
			PhotoID:     photoID,
			ImageVector: embeddings[i],
			CreatedAt:   timeNow(),
			UpdatedAt:   timeNow(),
		}
	}
	s.mu.Unlock()

	// Add to vector index
	if s.index != nil {
		if err := s.index.AddBatch(photoIDs, embeddings); err != nil {
			return fmt.Errorf("failed to add to index: %w", err)
		}
	}

	return nil
}

// SearchByText searches photos by text query
func (s *SemanticSearch) SearchByText(ctx context.Context, query string, limit int) ([]SemanticSearchResult, error) {
	// Encode text query
	textEmbedding, err := s.clip.EncodeText(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to encode text: %w", err)
	}

	// Search in vector index
	if s.index != nil {
		results, err := s.index.Search(textEmbedding, limit)
		if err != nil {
			return nil, fmt.Errorf("index search failed: %w", err)
		}

		searchResults := make([]SemanticSearchResult, len(results))
		for i, r := range results {
			searchResults[i] = SemanticSearchResult{
				PhotoID:   r.PhotoID,
				Score:     r.Score,
				MatchType: "text",
			}
		}
		return searchResults, nil
	}

	// Fallback to brute force search
	return s.bruteForceSearch(textEmbedding, limit)
}

// SearchByImage searches photos similar to given image
func (s *SemanticSearch) SearchByImage(ctx context.Context, img image.Image, limit int) ([]SemanticSearchResult, error) {
	// Encode query image
	queryEmbedding, err := s.clip.EncodeImage(ctx, img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	// Search in vector index
	if s.index != nil {
		results, err := s.index.Search(queryEmbedding, limit)
		if err != nil {
			return nil, fmt.Errorf("index search failed: %w", err)
		}

		searchResults := make([]SemanticSearchResult, len(results))
		for i, r := range results {
			searchResults[i] = SemanticSearchResult{
				PhotoID:   r.PhotoID,
				Score:     r.Score,
				MatchType: "image",
			}
		}
		return searchResults, nil
	}

	// Fallback to brute force search
	return s.bruteForceSearch(queryEmbedding, limit)
}

// bruteForceSearch performs brute force similarity search
func (s *SemanticSearch) bruteForceSearch(query []float32, limit int) ([]SemanticSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scoredPhoto struct {
		photoID string
		score   float64
	}

	scores := make([]scoredPhoto, 0, len(s.photos))
	for photoID, pv := range s.photos {
		sim, err := s.clip.ComputeSimilarity(query, pv.ImageVector)
		if err != nil {
			continue
		}
		scores = append(scores, scoredPhoto{photoID, sim})
	}

	// Sort by score descending
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	// Return top results
	if limit > len(scores) {
		limit = len(scores)
	}

	results := make([]SemanticSearchResult, limit)
	for i := 0; i < limit; i++ {
		results[i] = SemanticSearchResult{
			PhotoID:   scores[i].photoID,
			Score:     scores[i].score,
			MatchType: "hybrid",
		}
	}

	return results, nil
}

// RemovePhoto removes a photo from the index
func (s *SemanticSearch) RemovePhoto(photoID string) error {
	s.mu.Lock()
	delete(s.photos, photoID)
	s.mu.Unlock()

	if s.index != nil {
		return s.index.Delete(photoID)
	}
	return nil
}

// timeNow returns current time (for testing)
var timeNow = func() time.Time {
	return time.Now()
}
package ragserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler RAG服务器API handler.
type Handler struct {
	server *RAGServer
}

// NewHandler 创建handler.
func NewHandler(server *RAGServer) *Handler {
	return &Handler{server: server}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rag := rg.Group("/rag")
	{
		rag.POST("/collection", h.CreateCollection)
		rag.POST("/document", h.AddDocument)
		rag.POST("/query", h.Query)
		rag.GET("/collections", h.ListCollections)
	}
}

type CreateCollectionRequest struct {
	Name         string `json:"name" binding:"required"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
}

func (h *Handler) CreateCollection(c *gin.Context) {
	var req CreateCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.server.CreateCollection(c.Request.Context(), req.Name, req.ChunkSize, req.ChunkOverlap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

type AddDocumentRequest struct {
	Collection string            `json:"collection" binding:"required"`
	ID         string            `json:"id" binding:"required"`
	Content    string            `json:"content" binding:"required"`
	Metadata   map[string]string `json:"metadata"`
}

func (h *Handler) AddDocument(c *gin.Context) {
	var req AddDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc := &Document{
		ID:       req.ID,
		Content:  req.Content,
		Metadata: req.Metadata,
	}

	if err := h.server.AddDocument(c.Request.Context(), req.Collection, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "added", "chunks": len(doc.Chunks)})
}

type QueryReq struct {
	Collection string  `json:"collection" binding:"required"`
	Query      string  `json:"query" binding:"required"`
	TopK       int     `json:"top_k"`
	Threshold  float64 `json:"threshold"`
}

func (h *Handler) Query(c *gin.Context) {
	var req QueryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.server.Query(c.Request.Context(), &QueryRequest{
		Collection: req.Collection,
		Query:      req.Query,
		TopK:       req.TopK,
		Threshold:  req.Threshold,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListCollections(c *gin.Context) {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	var names []string
	for name := range h.server.collections {
		names = append(names, name)
	}

	c.JSON(http.StatusOK, gin.H{"collections": names})
}

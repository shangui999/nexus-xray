package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"gorm.io/gorm"
)

// NodeHandler 节点管理处理器
type NodeHandler struct {
	db         *gorm.DB
	serverAddr string
}

// NewNodeHandler 创建节点管理处理器
func NewNodeHandler(db *gorm.DB, serverAddr string) *NodeHandler {
	return &NodeHandler{db: db, serverAddr: serverAddr}
}

type createNodeRequest struct {
	Name             string `json:"name" binding:"required"`
	Address          string `json:"address" binding:"required"`
	SubscriptionAddr string `json:"subscription_addr"`
	IPv6Addr         string `json:"ipv6_addr"`
}

type updateNodeRequest struct {
	Name             *string `json:"name"`
	Address          *string `json:"address"`
	SubscriptionAddr *string `json:"subscription_addr"`
	IPv6Addr         *string `json:"ipv6_addr"`
}

type createNodeResponse struct {
	Node            model.Node `json:"node"`
	EnrollmentToken string     `json:"enrollment_token"`
	InstallCommand  string     `json:"install_command"`
}

// List 返回所有节点列表
// GET /api/nodes
func (h *NodeHandler) List(c *gin.Context) {
	var nodes []model.Node
	if err := h.db.Find(&nodes).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list nodes")
		return
	}
	Success(c, nodes)
}

// Create 创建节点，自动生成 enrollment token，返回安装命令
// POST /api/nodes
func (h *NodeHandler) Create(c *gin.Context) {
	var req createNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	node := model.Node{
		Name:             req.Name,
		Address:          req.Address,
		SubscriptionAddr: req.SubscriptionAddr,
		IPv6Addr:         req.IPv6Addr,
		Status:           "offline",
	}

	if err := h.db.Create(&node).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to create node")
		return
	}

	// 生成 enrollment token（使用 UUID 作为明文 token，存储 SHA256 哈希）
	token := uuid.New().String()
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	enrollmentToken := model.EnrollmentToken{
		NodeID:    node.ID,
		TokenHash: tokenHash,
		Used:      false,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := h.db.Create(&enrollmentToken).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to create enrollment token")
		return
	}

	installCmd := fmt.Sprintf(
		"curl -sL https://raw.githubusercontent.com/xray-manager/xray-manager/main/scripts/install-agent.sh | bash -s -- --server=%s --node-id=%s --token=%s",
		h.serverAddr, node.ID.String(), token,
	)

	Success(c, createNodeResponse{
		Node:            node,
		EnrollmentToken: token,
		InstallCommand:  installCmd,
	})
}

// Delete 软删除节点
// DELETE /api/nodes/:id
func (h *NodeHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid node ID")
		return
	}

	if err := h.db.Delete(&model.Node{}, nodeID).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to delete node")
		return
	}

	Success(c, nil)
}

// Update 更新节点信息
// PUT /api/nodes/:id
func (h *NodeHandler) Update(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid node ID")
		return
	}

	var req updateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	var node model.Node
	if err := h.db.First(&node, nodeID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(c, http.StatusNotFound, 404, "node not found")
			return
		}
		Error(c, http.StatusInternalServerError, 500, "failed to get node")
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Address != nil {
		updates["address"] = *req.Address
	}
	if req.SubscriptionAddr != nil {
		updates["subscription_addr"] = *req.SubscriptionAddr
	}
	if req.IPv6Addr != nil {
		updates["ipv6_addr"] = *req.IPv6Addr
	}

	if len(updates) > 0 {
		if err := h.db.Model(&node).Updates(updates).Error; err != nil {
			Error(c, http.StatusInternalServerError, 500, "failed to update node")
			return
		}
	}

	// 重新查询以获取更新后的数据
	if err := h.db.First(&node, nodeID).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to get node")
		return
	}

	Success(c, node)
}

// GetStatus 返回节点详细状态
// GET /api/nodes/:id/status
func (h *NodeHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid node ID")
		return
	}

	var node model.Node
	if err := h.db.First(&node, nodeID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(c, http.StatusNotFound, 404, "node not found")
			return
		}
		Error(c, http.StatusInternalServerError, 500, "failed to get node")
		return
	}

	// 获取最近 24h 流量统计
	var totalUpload int64
	var totalDownload int64
	h.db.Model(&model.TrafficLog{}).
		Where("node_id = ? AND recorded_at > ?", nodeID, time.Now().Add(-24*time.Hour)).
		Select("COALESCE(SUM(upload_bytes), 0)").
		Scan(&totalUpload)
	h.db.Model(&model.TrafficLog{}).
		Where("node_id = ? AND recorded_at > ?", nodeID, time.Now().Add(-24*time.Hour)).
		Select("COALESCE(SUM(download_bytes), 0)").
		Scan(&totalDownload)

	type nodeStatus struct {
		Node        model.Node `json:"node"`
		Upload24h   int64      `json:"upload_24h"`
		Download24h int64      `json:"download_24h"`
	}

	Success(c, nodeStatus{
		Node:        node,
		Upload24h:   totalUpload,
		Download24h: totalDownload,
	})
}

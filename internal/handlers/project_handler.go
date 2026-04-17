package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// ---------------------------------------------------------------------------
// ProjectHandler — Project CRUD（CICD_ARCHITECTURE §M14.1）
//
// PlatformAdmin 專用：管理 Git Provider 下的 Project（程式碼倉庫）。
// 路由前綴：/system/git-providers/:id/projects
// ---------------------------------------------------------------------------

// ProjectHandler 管理 Project CRUD API。
type ProjectHandler struct {
	projectSvc     *services.ProjectService
	gitProviderSvc *services.GitProviderService
}

// NewProjectHandler 建立 ProjectHandler。
func NewProjectHandler(projectSvc *services.ProjectService, gitProviderSvc *services.GitProviderService) *ProjectHandler {
	return &ProjectHandler{projectSvc: projectSvc, gitProviderSvc: gitProviderSvc}
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// CreateProjectRequest 建立 Project 的請求。
type CreateProjectRequest struct {
	Name          string `json:"name"           binding:"required,max=255"`
	RepoURL       string `json:"repo_url"       binding:"required,url,max=512"`
	DefaultBranch string `json:"default_branch" binding:"max=255"`
	Description   string `json:"description"`
}

// UpdateProjectRequest 更新 Project 的請求。
type UpdateProjectRequest struct {
	Name          *string `json:"name,omitempty"`
	RepoURL       *string `json:"repo_url,omitempty"`
	DefaultBranch *string `json:"default_branch,omitempty"`
	Description   *string `json:"description,omitempty"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// List 列出某 Git Provider 下的所有 Projects。
// GET /system/git-providers/:id/projects
func (h *ProjectHandler) List(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider ID")
		return
	}

	projects, err := h.projectSvc.ListProjects(c.Request.Context(), uint(providerID))
	if err != nil {
		response.InternalError(c, "failed to list projects: "+err.Error())
		return
	}
	response.List(c, projects, int64(len(projects)))
}

// Get 取得單一 Project。
// GET /system/git-providers/:id/projects/:projectID
func (h *ProjectHandler) Get(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("projectID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid project ID")
		return
	}

	project, err := h.projectSvc.GetProject(c.Request.Context(), uint(projectID))
	if err != nil {
		response.NotFound(c, "project not found")
		return
	}
	response.OK(c, project)
}

// Create 建立新 Project。
// POST /system/git-providers/:id/projects
func (h *ProjectHandler) Create(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider ID")
		return
	}

	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	userID := c.GetUint("user_id")
	defaultBranch := req.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// Validate git repo is reachable via the provider's API
	provider, err := h.gitProviderSvc.GetProvider(c.Request.Context(), uint(providerID))
	if err != nil {
		response.NotFound(c, "git provider not found")
		return
	}
	if err := h.gitProviderSvc.ValidateRepoConnection(c.Request.Context(), provider, req.RepoURL); err != nil {
		response.BadRequest(c, "git repo validation failed: "+err.Error())
		return
	}

	project := &models.Project{
		GitProviderID: uint(providerID),
		Name:          req.Name,
		RepoURL:       req.RepoURL,
		DefaultBranch: defaultBranch,
		Description:   req.Description,
		CreatedBy:     userID,
	}

	if err := h.projectSvc.CreateProject(c.Request.Context(), project); err != nil {
		response.InternalError(c, "failed to create project: "+err.Error())
		return
	}
	response.OK(c, project)
}

// Update 更新 Project。
// PUT /system/git-providers/:id/projects/:projectID
func (h *ProjectHandler) Update(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("projectID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid project ID")
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.RepoURL != nil {
		updates["repo_url"] = *req.RepoURL
	}
	if req.DefaultBranch != nil {
		updates["default_branch"] = *req.DefaultBranch
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}

	project, err := h.projectSvc.UpdateProject(c.Request.Context(), uint(projectID), updates)
	if err != nil {
		response.InternalError(c, "failed to update project: "+err.Error())
		return
	}
	response.OK(c, project)
}

// Delete 刪除 Project。
// DELETE /system/git-providers/:id/projects/:projectID
func (h *ProjectHandler) Delete(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("projectID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid project ID")
		return
	}

	if err := h.projectSvc.DeleteProject(c.Request.Context(), uint(projectID)); err != nil {
		response.InternalError(c, "failed to delete project: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

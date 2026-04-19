package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/service"
)

type agentRequestBody struct {
	Protocol string `json:"protocol"`
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
	Model    string `json:"model"`
}

func (h *Handler) ListAgents(c *gin.Context) {
	items, err := h.agents.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) UpdateAgent(c *gin.Context) {
	name := domain.AgentName(c.Param("name"))
	if !name.Valid() {
		writeError(c, service.ErrAgentNotFound)
		return
	}
	var body agentRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, service.ErrInvalidRequest)
		return
	}
	updated, err := h.agents.Update(c.Request.Context(), name, service.AgentUpdateInput{
		Protocol: domain.AgentProtocol(body.Protocol),
		BaseURL:  body.BaseURL,
		APIKey:   body.APIKey,
		Model:    body.Model,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) FetchAgentModels(c *gin.Context) {
	name := domain.AgentName(c.Param("name"))
	if !name.Valid() {
		writeError(c, service.ErrAgentNotFound)
		return
	}
	var body agentRequestBody
	_ = c.ShouldBindJSON(&body)
	models, err := h.agents.FetchModels(c.Request.Context(), name, service.FetchModelsInput{
		BaseURL: body.BaseURL,
		APIKey:  body.APIKey,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

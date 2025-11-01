package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gin-gonic/gin"
)

type Client struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	AIAPIKey  *string `json:"ai_api_key"`
	AIAPIURL  *string `json:"ai_api_url"`
	APIModel  *string `json:"ai_api_model"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
}

type Domain struct {
	ID        string `json:"id"`
	ClientID  string `json:"client_id"`
	Domain    string `json:"domain"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

type CreateClientRequest struct {
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	AIAPIKey *string `json:"ai_api_key"`
	AIAPIURL *string `json:"ai_api_url"`
	APIModel *string `json:"ai_api_model"`
}

type UpdateClientRequest struct {
	Name     *string `json:"name"`
	Slug     *string `json:"slug"`
	AIAPIKey *string `json:"ai_api_key"`
	AIAPIURL *string `json:"ai_api_url"`
	APIModel *string `json:"ai_api_model"`
	IsActive *bool   `json:"is_active"`
}

type CreateDomainRequest struct {
	ClientID string `json:"client_id"`
	Domain   string `json:"domain"`
}

type UpdateDomainRequest struct {
	Domain   *string `json:"domain"`
	IsActive *bool   `json:"is_active"`
}

func (app *App) getClientsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	resultSet, err := app.ZDB.Query(ctx,
		"SELECT id, name, slug, ai_api_key, ai_api_url, ai_api_model, is_active, created_at FROM clients ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch clients"})
		return
	}

	var clients []Client
	for _, row := range resultSet.Rows {
		if len(row.Values) < 8 {
			continue
		}
		
		var client Client
		if id, err := row.Values[0].GetString(); err == nil {
			client.ID = id
		}
		if name, err := row.Values[1].GetString(); err == nil {
			client.Name = name
		}
		if slug, err := row.Values[2].GetString(); err == nil {
			client.Slug = slug
		}
		if aiAPIKey, err := row.Values[3].GetString(); err == nil {
			client.AIAPIKey = &aiAPIKey
		}
		if aiAPIURL, err := row.Values[4].GetString(); err == nil {
			client.AIAPIURL = &aiAPIURL
		}
		if aiAPIModel, err := row.Values[5].GetString(); err == nil {
			client.APIModel = &aiAPIModel
		}
		if isActive, err := row.Values[6].GetBool(); err == nil {
			client.IsActive = isActive
		}
		if createdAt, err := row.Values[7].GetTime(); err == nil {
			client.CreatedAt = createdAt.Format(time.RFC3339)
		}
		
		clients = append(clients, client)
	}

	c.JSON(http.StatusOK, clients)
}

func (app *App) createClientHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client name is required"})
		return
	}

	if req.Slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client slug is required"})
		return
	}

	// Check if slug already exists using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM clients WHERE slug = $1)",
		req.Slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	var exists bool
	if err := row.Values[0].GetBool(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
		return
	}
	
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Client slug already exists"})
		return
	}

	clientID := uuid.New().String()
	_, err = app.ZDB.Execute(ctx,
		"INSERT INTO clients (id, name, slug, ai_api_key, ai_api_url, ai_api_model, is_active, created_at) VALUES ($1, $2, $3, $4, $5, $6, true, CURRENT_TIMESTAMP)",
		clientID, req.Name, req.Slug, req.AIAPIKey, req.AIAPIURL, req.APIModel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create client"})
		return
	}
	
	// Get created timestamp using ZDB
	row, err = app.ZDB.QueryRow(ctx,
		"SELECT created_at FROM clients WHERE id = $1",
		clientID)
	if err != nil || len(row.Values) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get client details"})
		return
	}
	
	createdAt, err := row.Values[0].GetTime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse timestamp"})
		return
	}

	client := Client{
		ID:        clientID,
		Name:      req.Name,
		Slug:      req.Slug,
		AIAPIKey:  req.AIAPIKey,
		AIAPIURL:  req.AIAPIURL,
		APIModel:  req.APIModel,
		IsActive:  true,
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, client)
}

func (app *App) updateClientHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clientID := c.Param("id")

	var req UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Check if client exists using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)",
		clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	var exists bool
	if err := row.Values[0].GetBool(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
		return
	}
	
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}

	// If updating slug, check for uniqueness using ZDB
	if req.Slug != nil {
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM clients WHERE slug = $1 AND id != $2)",
			*req.Slug, clientID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		
		var slugExists bool
		if err := row.Values[0].GetBool(&slugExists); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
			return
		}
		
		if slugExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Client slug already exists"})
			return
		}
	}

	// Build dynamic update query
	query := "UPDATE clients SET updated_at = CURRENT_TIMESTAMP"
	args := []interface{}{}
	argIndex := 1

	if req.Name != nil {
		query += fmt.Sprintf(", name = $%d", argIndex)
		args = append(args, *req.Name)
		argIndex++
	}

	if req.Slug != nil {
		query += fmt.Sprintf(", slug = $%d", argIndex)
		args = append(args, *req.Slug)
		argIndex++
	}

	if req.AIAPIKey != nil {
		query += fmt.Sprintf(", ai_api_key = $%d", argIndex)
		args = append(args, *req.AIAPIKey)
		argIndex++
	}

	if req.AIAPIURL != nil {
		query += fmt.Sprintf(", ai_api_url = $%d", argIndex)
		args = append(args, *req.AIAPIURL)
		argIndex++
	}

	if req.APIModel != nil {
		query += fmt.Sprintf(", ai_api_model = $%d", argIndex)
		args = append(args, *req.APIModel)
		argIndex++
	}

	if req.IsActive != nil {
		query += fmt.Sprintf(", is_active = $%d", argIndex)
		args = append(args, *req.IsActive)
		argIndex++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIndex)
	args = append(args, clientID)

	_, err = app.ZDB.Execute(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update client"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Client updated successfully"})
}

func (app *App) deleteClientHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clientID := c.Param("id")

	// Soft delete by setting is_active to false using ZDB
	result, err := app.ZDB.Execute(ctx,
		"UPDATE clients SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = $1",
		clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete client"})
		return
	}

	rowsAffected := result.RowsAffected
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Client deleted successfully"})
}

func (app *App) getDomainsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	clientID := c.Query("client_id")

	var query string
	var args []interface{}

	if clientID != "" {
		query = "SELECT id, client_id, domain, is_active, created_at FROM domains WHERE client_id = $1 ORDER BY created_at DESC"
		args = []interface{}{clientID}
	} else {
		query = "SELECT id, client_id, domain, is_active, created_at FROM domains ORDER BY created_at DESC"
		args = []interface{}{}
	}

	resultSet, err := app.ZDB.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domains"})
		return
	}

	var domains []Domain
	for _, row := range resultSet.Rows {
		if len(row.Values) < 5 {
			continue
		}
		
		var domain Domain
		if id, err := row.Values[0].GetString(); err == nil {
			domain.ID = id
		}
		if clientID, err := row.Values[1].GetString(); err == nil {
			domain.ClientID = clientID
		}
		if domainName, err := row.Values[2].GetString(); err == nil {
			domain.Domain = domainName
		}
		if isActive, err := row.Values[3].GetBool(); err == nil {
			domain.IsActive = isActive
		}
		if createdAt, err := row.Values[4].GetTime(); err == nil {
			domain.CreatedAt = createdAt.Format(time.RFC3339)
		}
		
		domains = append(domains, domain)
	}

	c.JSON(http.StatusOK, domains)
}

func (app *App) createDomainHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if req.ClientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client ID is required"})
		return
	}

	if req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Domain is required"})
		return
	}

	// Check if client exists
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)",
		req.ClientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	var clientExists bool
	if err := row.Values[0].GetBool(&clientExists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
		return
	}
	
	if !clientExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
		return
	}

	// Check if domain already exists
	row, err = app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM domains WHERE domain = $1)",
		req.Domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	var domainExists bool
	if err := row.Values[0].GetBool(&domainExists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
		return
	}
	
	if domainExists {
		c.JSON(http.StatusConflict, gin.H{"error": "Domain already exists"})
		return
	}

	domainID := uuid.New().String()
	row, err = app.ZDB.QueryRow(ctx,
		"INSERT INTO domains (id, client_id, domain, is_active, created_at) VALUES ($1, $2, $3, true, CURRENT_TIMESTAMP) RETURNING created_at",
		domainID, req.ClientID, req.Domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
		return
	}
	
	createdAt, err := row.Values[0].GetTime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse timestamp"})
		return
	}

	domain := Domain{
		ID:        domainID,
		ClientID:  req.ClientID,
		Domain:    req.Domain,
		IsActive:  true,
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, domain)
}

func (app *App) updateDomainHandler(c *gin.Context) {
	ctx := c.Request.Context()
	domainID := c.Param("id")

	var req UpdateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Check if domain exists
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM domains WHERE id = $1)",
		domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	var exists bool
	if err := row.Values[0].GetBool(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
		return
	}
	
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// If updating domain, check for uniqueness
	if req.Domain != nil {
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM domains WHERE domain = $1 AND id != $2)",
			*req.Domain, domainID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		
		var domainExists bool
		if err := row.Values[0].GetBool(&domainExists); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
			return
		}
		
		if domainExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Domain already exists"})
			return
		}
	}

	// Build dynamic update query
	query := "UPDATE domains SET updated_at = CURRENT_TIMESTAMP"
	args := []interface{}{}
	argIndex := 1

	if req.Domain != nil {
		query += fmt.Sprintf(", domain = $%d", argIndex)
		args = append(args, *req.Domain)
		argIndex++
	}

	if req.IsActive != nil {
		query += fmt.Sprintf(", is_active = $%d", argIndex)
		args = append(args, *req.IsActive)
		argIndex++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIndex)
	args = append(args, domainID)

	_, err = app.ZDB.Execute(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update domain"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Domain updated successfully"})
}

func (app *App) deleteDomainHandler(c *gin.Context) {
	ctx := c.Request.Context()
	domainID := c.Param("id")

	// Soft delete by setting is_active to false
	result, err := app.ZDB.Execute(ctx,
		"UPDATE domains SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = $1",
		domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete domain"})
		return
	}

	rowsAffected := result.RowsAffected
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted successfully"})
}

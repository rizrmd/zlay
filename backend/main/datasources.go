package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gin-gonic/gin"
)

type Datasource struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	IsActive  bool            `json:"is_active"`
	CreatedAt string          `json:"created_at"`
}

type CreateDatasourceRequest struct {
	ProjectID string          `json:"project_id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
}

type UpdateDatasourceRequest struct {
	Name     *string          `json:"name"`
	Type     *string          `json:"type"`
	Config   *json.RawMessage `json:"config"`
	IsActive *bool            `json:"is_active"`
}

func (app *App) getDatasourcesHandler(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetString("user_id")

	// Get project ID from query param or check if user has access to all projects
	projectID := c.Query("project_id")

	var query string
	var args []interface{}

	if projectID != "" {
		// Check if user owns the project using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1 AND user_id = $2 AND is_active = true)",
			projectID, userID)
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found or no access"})
			return
		}

		query = "SELECT id, project_id, name, type, config, is_active, created_at FROM datasources WHERE project_id = $1 AND is_active = true ORDER BY created_at DESC"
		args = []interface{}{projectID}
	} else {
		// Get all datasources for user's projects
		query = `SELECT d.id, d.project_id, d.name, d.type, d.config, d.is_active, d.created_at 
				 FROM datasources d 
				 JOIN projects p ON d.project_id = p.id 
				 WHERE p.user_id = $1 AND d.is_active = true AND p.is_active = true 
				 ORDER BY d.created_at DESC`
		args = []interface{}{userID}
	}

	resultSet, err := app.ZDB.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch datasources"})
		return
	}

	var datasources []Datasource
	for _, row := range resultSet.Rows {
		if len(row.Values) < 7 {
			continue
		}
		
		var datasource Datasource
		if id, err := row.Values[0].GetString(); err == nil {
			datasource.ID = id
		}
		if projectID, err := row.Values[1].GetString(); err == nil {
			datasource.ProjectID = projectID
		}
		if name, err := row.Values[2].GetString(); err == nil {
			datasource.Name = name
		}
		if datasourceType, err := row.Values[3].GetString(); err == nil {
			datasource.Type = datasourceType
		}
		if config, err := row.Values[4].GetRaw(); err == nil {
			datasource.Config = config
		}
		if isActive, err := row.Values[5].GetBool(); err == nil {
			datasource.IsActive = isActive
		}
		if createdAt, err := row.Values[6].GetTime(); err == nil {
			datasource.CreatedAt = createdAt.Format(time.RFC3339)
		}
		
		datasources = append(datasources, datasource)
	}

	c.JSON(http.StatusOK, datasources)
}

func (app *App) createDatasourceHandler(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetString("user_id")

	var req CreateDatasourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datasource name is required"})
		return
	}

	if req.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datasource type is required"})
		return
	}

	// Check if user owns the project using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1 AND user_id = $2 AND is_active = true)",
		req.ProjectID, userID)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found or no access"})
		return
	}

	datasourceID := uuid.New().String()
	_, err = app.ZDB.Execute(ctx,
		"INSERT INTO datasources (id, project_id, name, type, config, is_active, created_at) VALUES ($1, $2, $3, $4, $5, true, CURRENT_TIMESTAMP)",
		datasourceID, req.ProjectID, req.Name, req.Type, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create datasource"})
		return
	}
	
	// Get created timestamp using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT created_at FROM datasources WHERE id = $1",
		datasourceID)
	if err != nil || len(row.Values) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get datasource details"})
		return
	}
	
	createdAt, err := row.Values[0].GetTime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse timestamp"})
		return
	}

	datasource := Datasource{
		ID:        datasourceID,
		ProjectID: req.ProjectID,
		Name:      req.Name,
		Type:      req.Type,
		Config:    req.Config,
		IsActive:  true,
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, datasource)
}

func (app *App) getDatasourceHandler(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetString("user_id")
	datasourceID := c.Param("id")

	row, err := app.ZDB.QueryRow(ctx,
		`SELECT d.id, d.project_id, d.name, d.type, d.config, d.is_active, d.created_at 
		 FROM datasources d 
		 JOIN projects p ON d.project_id = p.id 
		 WHERE d.id = $1 AND p.user_id = $2 AND d.is_active = true AND p.is_active = true`,
		datasourceID, userID)
	if err != nil || len(row.Values) < 7 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Datasource not found"})
		return
	}
	
	var datasource Datasource
	if id, err := row.Values[0].GetString(); err == nil {
		datasource.ID = id
	}
	if projectID, err := row.Values[1].GetString(); err == nil {
		datasource.ProjectID = projectID
	}
	if name, err := row.Values[2].GetString(); err == nil {
		datasource.Name = name
	}
	if datasourceType, err := row.Values[3].GetString(); err == nil {
		datasource.Type = datasourceType
	}
	if config, err := row.Values[4].GetRaw(); err == nil {
		datasource.Config = config
	}
	if isActive, err := row.Values[5].GetBool(); err == nil {
		datasource.IsActive = isActive
	}
	if createdAt, err := row.Values[6].GetTime(); err == nil {
		datasource.CreatedAt = createdAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, datasource)
}

func (app *App) updateDatasourceHandler(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetString("user_id")
	datasourceID := c.Param("id")

	var req UpdateDatasourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Check if datasource exists and user has access using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM datasources d 
		 JOIN projects p ON d.project_id = p.id 
		 WHERE d.id = $1 AND p.user_id = $2 AND d.is_active = true AND p.is_active = true)`,
		datasourceID, userID)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Datasource not found"})
		return
	}

	// Build dynamic update query
	query := "UPDATE datasources SET updated_at = CURRENT_TIMESTAMP"
	args := []interface{}{}
	argIndex := 1

	if req.Name != nil {
		query += fmt.Sprintf(", name = $%d", argIndex)
		args = append(args, *req.Name)
		argIndex++
	}

	if req.Type != nil {
		query += fmt.Sprintf(", type = $%d", argIndex)
		args = append(args, *req.Type)
		argIndex++
	}

	if req.Config != nil {
		query += fmt.Sprintf(", config = $%d", argIndex)
		args = append(args, *req.Config)
		argIndex++
	}

	if req.IsActive != nil {
		query += fmt.Sprintf(", is_active = $%d", argIndex)
		args = append(args, *req.IsActive)
		argIndex++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIndex)
	args = append(args, datasourceID)

	_, err = app.ZDB.Execute(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update datasource"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Datasource updated successfully"})
}

func (app *App) deleteDatasourceHandler(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetString("user_id")
	datasourceID := c.Param("id")

	// Soft delete by setting is_active to false using ZDB
	result, err := app.ZDB.Execute(ctx,
		`UPDATE datasources d 
		 SET is_active = false, updated_at = CURRENT_TIMESTAMP 
		 FROM projects p 
		 WHERE d.id = $1 AND p.id = d.project_id AND p.user_id = $2 AND d.is_active = true`,
		datasourceID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete datasource"})
		return
	}

	rowsAffected := result.RowsAffected
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Datasource not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Datasource deleted successfully"})
}

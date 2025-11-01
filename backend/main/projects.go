package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Project struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

func (app *App) getProjectsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Get current user
	user, err := app.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID := user.ID

	resultSet, err := app.ZDB.Query(ctx,
		"SELECT id, user_id, name, description, is_active, created_at FROM projects WHERE user_id = $1 AND is_active = true ORDER BY created_at DESC",
		userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch projects"})
		return
	}

	var projects []Project
	for _, row := range resultSet.Rows {
		if len(row.Values) < 6 {
			continue
		}

		var project Project
		if id, ok := row.Values[0].AsString(); ok {
			project.ID = id
		}
		if userID, ok := row.Values[1].AsString(); ok {
			project.UserID = userID
		}
		if name, ok := row.Values[2].AsString(); ok {
			project.Name = name
		}
		if description, ok := row.Values[3].AsString(); ok {
			project.Description = description
		}
		if isActive, ok := row.Values[4].AsBool(); ok {
			project.IsActive = isActive
		}
		if createdAt, ok := row.Values[5].AsTimestamp(); ok {
			project.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}

		projects = append(projects, project)
	}

	c.JSON(http.StatusOK, projects)
}

func (app *App) createProjectHandler(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Get current user
	user, err := app.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID := user.ID

	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required"})
		return
	}

	projectID := uuid.New().String()
	row, err := app.ZDB.QueryRow(ctx,
		"INSERT INTO projects (id, user_id, name, description, is_active, created_at) VALUES ($1, $2, $3, $4, true, CURRENT_TIMESTAMP) RETURNING created_at",
		projectID, userID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	createdAt, ok := row.Values[0].AsTimestamp()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse timestamp"})
		return
	}

	project := Project{
		ID:          projectID,
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
		CreatedAt:   createdAt.Time.Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, project)
}

func (app *App) getProjectHandler(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Get current user
	user, err := app.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID := user.ID
	projectID := c.Param("id")

	row, err := app.ZDB.QueryRow(ctx,
		"SELECT id, user_id, name, description, is_active, created_at FROM projects WHERE id = $1 AND user_id = $2 AND is_active = true",
		projectID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if len(row.Values) < 6 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	var project Project
	var ok bool
	project.ID, ok = row.Values[0].AsString()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse project ID"})
		return
	}
	project.UserID, ok = row.Values[1].AsString()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user ID"})
		return
	}
	project.Name, ok = row.Values[2].AsString()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse project name"})
		return
	}
	project.Description, ok = row.Values[3].AsString()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse project description"})
		return
	}
	project.IsActive, ok = row.Values[4].AsBool()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse active status"})
		return
	}
	if createdAt, ok := row.Values[5].AsTimestamp(); ok {
		project.CreatedAt = createdAt.Time.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, project)
}

func (app *App) updateProjectHandler(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Get current user
	user, err := app.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID := user.ID
	projectID := c.Param("id")

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Check if project exists and belongs to user
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1 AND user_id = $2 AND is_active = true)",
		projectID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	exists, ok := row.Values[0].AsBool()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Build dynamic update query
	query := "UPDATE projects SET updated_at = CURRENT_TIMESTAMP"
	args := []interface{}{}
	argIndex := 1

	if req.Name != nil {
		query += fmt.Sprintf(", name = $%d", argIndex)
		args = append(args, *req.Name)
		argIndex++
	}

	if req.Description != nil {
		query += fmt.Sprintf(", description = $%d", argIndex)
		args = append(args, *req.Description)
		argIndex++
	}

	if req.IsActive != nil {
		query += fmt.Sprintf(", is_active = $%d", argIndex)
		args = append(args, *req.IsActive)
		argIndex++
	}

	query += fmt.Sprintf(" WHERE id = $%d AND user_id = $%d", argIndex, argIndex+1)
	args = append(args, projectID, userID)

	_, err = app.ZDB.Execute(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project updated successfully"})
}

func (app *App) deleteProjectHandler(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Get current user
	user, err := app.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID := user.ID
	projectID := c.Param("id")

	// Soft delete by setting is_active to false
	result, err := app.ZDB.Execute(ctx,
		"UPDATE projects SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND user_id = $2 AND is_active = true",
		projectID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete project"})
		return
	}

	rowsAffected := result.RowsAffected
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deleted successfully"})
}

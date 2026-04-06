# Swagger API Documentation

## Setup

1. Install swag CLI:
   ```bash
   go install github.com/swaggo/swag/cmd/swag@latest
   ```

2. Generate Swagger documentation:
   ```bash
   make swagger
   ```
   Or manually:
   ```bash
   swag init -g cmd/server/main.go -o ./docs
   ```

3. Start the server:
   ```bash
   go run cmd/server/main.go
   ```

4. Open Swagger UI:
   ```
   http://localhost:8000/swagger/index.html
   ```

## Authentication

Most endpoints require authentication via cookie. To test authenticated endpoints:

1. First call `/api/login` with credentials:
   ```json
   {
     "username": "admin",
     "password": "admin"
   }
   ```

2. The cookie `wg_session` will be set automatically

3. Subsequent requests will use this cookie for authentication

## Updating Documentation

After modifying API handlers or adding new endpoints:

1. Update Swagger annotations in handler files
2. Regenerate documentation:
   ```bash
   make swagger
   ```
3. Restart the server

## Swagger Annotations

Swagger annotations use the `godoc` format:

```go
// GetNodes godoc
// @Summary      Get all nodes
// @Description  Retrieve list of all WireGuard nodes
// @Tags         nodes
// @Security     CookieAuth
// @Success      200  {array}   NodeItem
// @Failure      401  {object}  map[string]interface{}  "Unauthorized"
// @Router       /nodes [get]
func (h *DashboardHandler) GetNodes(c *gin.Context) {
    // ...
}
```

See [swaggo/swag](https://github.com/swaggo/swag) for more details.

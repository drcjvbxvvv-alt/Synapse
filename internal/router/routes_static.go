package router

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/response"
)

// setupStatic configures the embedded frontend static file serving.
func setupStatic(r *gin.Engine) {
	assetsFS, err := fs.Sub(staticFS, "ui/dist/assets")
	if err != nil {
		response.NotFound(nil, "")
		return
	}

	assetsGroup := r.Group("/assets")
	assetsGroup.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Next()
	})
	assetsGroup.StaticFS("/", http.FS(assetsFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
			response.NotFound(c, "not found")
			return
		}

		filePath := strings.TrimPrefix(path, "/")
		if filePath != "" {
			if f, err := staticFS.Open("ui/dist/" + filePath); err == nil {
				_ = f.Close()
				fileServer := http.FileServer(http.FS(mustSub(staticFS, "ui/dist")))
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		content, err := staticFS.ReadFile("ui/dist/index.html")
		if err != nil {
			response.InternalError(c, "frontend not available")
			return
		}
		c.Data(200, "text/html; charset=utf-8", content)
	})
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(fmt.Sprintf("fs.Sub(%q): %v", dir, err))
	}
	return sub
}

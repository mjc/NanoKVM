package router

import (
	"NanoKVM-Server/service/download"
	"github.com/gin-gonic/gin"

	"NanoKVM-Server/middleware"
)

func downloadRouter(r *gin.Engine) {
	service := download.NewService()
	api := r.Group("/api")
	api.Use(middleware.CheckToken())

	api.POST("/download/image", service.DownloadImage)       // download image
	api.GET("/download/image/enabled", service.ImageEnabled) // download image
	stepUp := api.Group("/download").Use(middleware.CheckToken())
	stepUp.GET("/image/status", service.StatusImage)
	stepUp.POST("/file", service.DownloadImageFile)
}

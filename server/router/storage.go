package router

import (
	"github.com/gin-gonic/gin"

	"NanoKVM-Server/middleware"
	"NanoKVM-Server/service/storage"
)

func storageRouter(r *gin.Engine) {
	service := storage.NewService()
	api := r.Group("/api")
	api.Use(middleware.CheckToken())

	api.GET("/storage/image", service.GetImages)               // get image list
	api.GET("/storage/image/mounted", service.GetMountedImage) // get mounted image
	api.GET("/storage/cdrom", service.GetCdRom)                // get CD-ROM flag
	stepUp := api.Group("/storage").Use(middleware.CheckToken())
	stepUp.POST("/image/mount", service.MountImage)
	stepUp.POST("/image/delete", service.DeleteImage)
}

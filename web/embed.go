// Package web 把构建好的前端静态资源 embed 进二进制（单镜像，评审约束 2）。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist 返回以 dist/ 为根的文件系统（index.html 在其根部）。
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist 目录由 //go:embed 保证存在
	}
	return sub
}

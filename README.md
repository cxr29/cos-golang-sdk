# qcloud_cos-golang
golang sdk for [腾讯云对象存储服务](http://wiki.qcloud.com/wiki/COS%E4%BA%A7%E5%93%81%E4%BB%8B%E7%BB%8D)

[![GoDoc](https://godoc.org/github.com/cxr29/cos-golang-sdk?status.svg)](https://godoc.org/github.com/cxr29/cos-golang-sdk)
[![Build Status](https://drone.io/github.com/cxr29/cos-golang-sdk/status.png)](https://drone.io/github.com/cxr29/cos-golang-sdk/latest)

## 安装

$ go get github.com/cxr29/cos-golang-sdk

## 举例
```go
package main

import (
	"fmt"

	"github.com/cxr29/cos-golang-sdk"
)

func main() {
	c := cos.New("项目ID", "签名ID", "签名秘钥")

	f := c.Bucket("空间名称").Dir("目录名称").File("文件名称") // 链式调用

	// 上传本地文件
	result, err := f.Upload("本地文件名称", "文件属性") // 完整上传，适用于较小文件
	// result, err := f.Upload([]byte(`文件内容`), "文件属性") // 直接上传文件内容
	// result, _, err := f.UploadSlice("本地文件名称", "文件属性", 0, "") // 分片上传，适用于较大文件
	fmt.Println(result, err)

	// 查询文件状态
	stat, err := f.Stat()
	fmt.Println(stat, err)

	// 删除文件
	err = f.Delete()

	// 创建目录
	d := f.Dir().Bucket().Dir("新的目录名称") // 向上链式调用
	fmt.Println(d.Create("目录属性"))

	// 更新目录属性
	err = d.Update("新的目录属性")

	// 获取指定目录下文件列表
	fmt.Println(d.List(nil))

	ldp := cos.NewListDirParams().Num(10).Order(1) // 可复用，如重新设置前一页返回的透传字段

	// 获取指定目录下以"abc"开头的文件，每页10个且反序
	fmt.Println(d.PrefixSearch("abc", ldp))

	// 删除目录
	err = d.Delete()
}
```

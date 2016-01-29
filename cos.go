// Copyright 2016 Chen Xianren. All rights reserved.

// Package cos 腾讯云平台对象存储服务Golang开发包
package cos

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Version SDK版本号
const Version = "0.0.1"

var (
	// SliceSize 默认分片大小
	SliceSize = 512 * 1024
	// SignSeconds 默认多次签名过期秒数
	SignSeconds = 60
	// UserAgent 默认用户代理
	UserAgent = "Qcloud-Cos-GOLANG/" + Version +
		" (" + runtime.GOOS + "-" + runtime.GOARCH + "-" + runtime.Version() + ")"
	// EndPoint 结尾无斜杠
	EndPoint = "http://web.file.myqcloud.com/files/v1"
)

// COS 表示对象存储服务
type COS struct {
	appID               string
	secretID, secretKey string
}

// New 根据项目ID、签名ID和签名秘钥返回一个COS结构体
func New(appID, secretID, secretKey string) COS {
	return COS{appID, secretID, secretKey}
}

// Bucket 根据Bucket名称返回COS下的Bucket结构体
func (cos COS) Bucket(name string) Bucket {
	return Bucket{cos, name}
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

func (cos COS) sign(b string, e, t int64, r int, f string) string {
	s := fmt.Sprintf("a=%s&b=%s&k=%s&e=%d&t=%d&r=%d&f=%s",
		cos.appID, b, cos.secretID, e, t, r, f,
	)
	h := hmac.New(sha1.New, []byte(cos.secretKey))
	h.Write([]byte(s))
	return base64.StdEncoding.EncodeToString(append(h.Sum(nil), []byte(s)...))
}

// Reply 表示COS RESTful API的响应内容
type Reply struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *json.RawMessage `json:"data"`
}

func (r *Reply) Error() string {
	return fmt.Sprintf("code: %d, message: %s", r.Code, r.Message)
}

// Bucket 表示文件资源的组织管理单元
type Bucket struct {
	cos  COS
	name string
}

// Dir 根据目录名称返回Bucket下的目录结构体
//  支持多级目录；如果目录名称为空，则表示Bucket的根目录
func (b Bucket) Dir(name string) Dir {
	return Dir{b, name}
}

func (b Bucket) getResourcePath(path string) string {
	return strings.Join([]string{
		"",
		b.cos.appID,
		b.name,
		path,
	}, "/")
}

func (b Bucket) getURL(path string) string {
	return EndPoint + b.getResourcePath(path)
}

func (b Bucket) sign(path string, seconds int) string {
	t := time.Now().Unix()
	var e int64
	var f string
	if seconds > 0 {
		e = t + int64(seconds)
	} else {
		f = b.getResourcePath(path)
	}
	return b.cos.sign(b.name, e, t, random.Int(), f)
}

func newHeader(auth string) http.Header {
	return http.Header{"Authorization": []string{auth}}
}

func setHeader(header http.Header, contentType string, contentLength int) http.Header {
	if header == nil {
		header = make(http.Header, 2)
	}
	header.Set("Content-Type", contentType)
	header.Set("Content-Length", strconv.Itoa(contentLength))
	return header
}

func do(method, urlStr string, header http.Header, body io.Reader, data interface{}) error {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", UserAgent)
	for k, v := range header {
		req.Header[k] = v
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	reply := new(Reply)

	err = json.NewDecoder(res.Body).Decode(reply)
	if err != nil {
		return err
	}

	if reply.Code != 0 {
		return reply
	}

	if res.StatusCode != http.StatusOK {
		reply.Code = res.StatusCode
		return reply
	}

	if data != nil {
		if reply.Data != nil {
			return json.Unmarshal(*reply.Data, data)
		}

		reply.Code = -1
		reply.Message = "no data"
		return reply
	}

	return nil
}

func doJSON(method, urlStr string, header http.Header, body, data interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	header = setHeader(header, "application/json", len(b))
	return do(method, urlStr, header, bytes.NewReader(b), data)
}

func (b Bucket) get(path, auth string, query params, data interface{}) error {
	return do("GET", b.getURL(path)+"?"+query.Encode(), newHeader(auth), nil, data)
}

func (b Bucket) post(path, auth string, body params, data interface{}) error {
	return doJSON("POST", b.getURL(path), newHeader(auth), body, data)
}

// Dir 表示Bucket下的目录
type Dir struct {
	bucket Bucket
	name   string
}

// File 根据文件名称返回目录下的文件结构体
//  文件名称不能为空
func (d Dir) File(name string) File {
	return File{d, name}
}

// Bucket 返回目录所在的Bucket
func (d Dir) Bucket() Bucket {
	return d.bucket
}

type params map[string]string

func newParams(op string) params {
	return params{"op": op}
}

func (m params) Set(k, v string) params {
	m[k] = v
	return m
}

func (m params) Encode() string {
	a := make([]string, 0, len(m))
	for k, v := range m {
		a = append(a, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return strings.Join(a, "&")
}

func (d Dir) get(auth string, query params, data interface{}) error {
	return d.bucket.get(d.Name(), auth, query, data)
}

func (d Dir) post(auth string, body params, data interface{}) error {
	return d.bucket.post(d.Name(), auth, body, data)
}

// CreateDirResult 表示创建目录的返回数据
type CreateDirResult struct {
	Ctime        string `json:"ctime"`
	ResourcePath string `json:"resource_path"`
}

// Create 创建目录
//  bizAttr 目录属性
func (d Dir) Create(bizAttr string) (*CreateDirResult, error) {
	data := new(CreateDirResult)
	err := d.post(d.Sign(SignSeconds), newParams("create").Set("biz_attr", bizAttr), data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Update 更新目录信息
//  bizAttr 目录属性
func (d Dir) Update(bizAttr string) error {
	return d.post(d.Sign(0), newParams("update").Set("biz_attr", bizAttr), nil)
}

// Stat 查询目录信息
func (d Dir) Stat() (*PathInfo, error) {
	data := new(PathInfo)
	err := d.get(d.Sign(SignSeconds), newParams("stat"), data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Delete 删除目录
func (d Dir) Delete() error {
	return d.post(d.Sign(0), newParams("delete"), nil)
}

// ListDirParams 表示目录列表和前缀搜索的参数
type ListDirParams map[string]string

// NewListDirParams 返回目录列表和前缀搜索的默认参数
func NewListDirParams() ListDirParams {
	params := make(ListDirParams, 4)
	return params.Num(20).Pattern("eListBoth").Order(0).Context("")
}

// Num 查询的数量，默认20
func (m ListDirParams) Num(num int) ListDirParams {
	m["num"] = strconv.Itoa(num)
	return m
}

// Pattern 可选eListBoth或eListDirOnly或eListFileOnly，默认eListBoth
func (m ListDirParams) Pattern(pattern string) ListDirParams {
	m["pattern"] = pattern
	return m
}

// Order 可选0正序或1反序，默认0正序
func (m ListDirParams) Order(order int) ListDirParams {
	m["order"] = strconv.Itoa(order)
	return m
}

// Context 透传字段，查看第一页，则传空字符串；若翻页，需要将前一页返回的Context透传到参数中
func (m ListDirParams) Context(context string) ListDirParams {
	m["context"] = context
	return m
}

// ListDirResult 表示目录列表和前缀搜索的返回数据
type ListDirResult struct {
	Context   string `json:"context"`
	DirCount  int    `json:"dircount"`
	FileCount int    `json:"filecount"`
	HasMore   bool   `json:"has_more"`
	Infos     []*PathInfo
}

// List 目录列表
//  params 如果为nil，则使用默认参数
func (d Dir) List(params ListDirParams) (*ListDirResult, error) {
	return d.PrefixSearch("", params)
}

// PrefixSearch 前缀搜索
//  prefix 列出含此前缀的所有文件，如果为空则获取目录列表
//  params 如果为nil，则使用默认参数
func (d Dir) PrefixSearch(prefix string, params ListDirParams) (*ListDirResult, error) {
	if params == nil {
		params = NewListDirParams()
	}

	query := newParams("list")
	for _, k := range []string{"num", "pattern", "order", "context"} {
		query[k] = params[k]
	}

	data := new(ListDirResult)
	err := d.bucket.get(d.Name()+EscapePath(prefix), d.Sign(SignSeconds), query, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

var replacer = strings.NewReplacer(
	"%2F", "/", "%2f", "/",
	"%7E", "~", "%7e", "~",
)

// EscapePath 转义路径，斜杠(/)和波浪号(~)不转义，移除开头和结尾的斜杠(/)
func EscapePath(name string) string {
	return replacer.Replace(url.QueryEscape(strings.Trim(name, "/")))
}

// Name 返回已转义的目录名称；如果不为空，始终含末尾斜杠(/)
func (d Dir) Name() string {
	if p := EscapePath(d.name); p != "" {
		return p + "/"
	}
	return ""
}

// Sign 根据seconds过期秒数返回目录的多次(大于0时)签名或单次(小于等于0时)签名
func (d Dir) Sign(seconds int) string {
	return d.bucket.sign(d.Name(), seconds)
}

// File 表示文件对象
type File struct {
	dir  Dir
	name string
}

// Dir 返回文件所在目录
func (f File) Dir() Dir {
	return f.dir
}

func (f File) get(auth string, query params, data interface{}) error {
	return f.dir.bucket.get(f.FullName(), auth, query, data)
}

func (f File) post(auth string, body params, data interface{}) error {
	return f.dir.bucket.post(f.FullName(), auth, body, data)
}

// UploadResult 表示上传文件的返回数据
type UploadResult struct {
	AccessURL    string `json:"access_url"`
	ResourcePath string `json:"resource_path"`
	SourceURL    string `json:"source_url"`
	URL          string `json:"url"`
}

func sha1sum(data []byte) string {
	b := sha1.Sum(data)
	return strings.ToUpper(hex.EncodeToString(b[:]))
}

// Upload 根据内容直接上传，用于较小文件(一般小于8MB)
//  bizAttr 文件属性
//  如果content类型为string，作为本地文件名读取其内容后上传；如果需要直接上传字符串，请转换为[]byte或使用strings.Reader进行包装
//  如果content类型为[]byte，直接上传
//  如果content类型为*byte.Buffer，直接上传其缓冲区内容
//  如果content类型为io.Reader，读取内容后上传
//  不支持其它content类型
func (f File) Upload(content interface{}, bizAttr string) (*UploadResult, error) {
	var err error
	var b []byte

	switch x := content.(type) {
	case string:
		b, err = ioutil.ReadFile(x)
		if err != nil {
			return nil, err
		}
	case []byte:
		b = x
	case *bytes.Buffer:
		b = x.Bytes()
	case io.Reader:
		b, err = ioutil.ReadAll(x)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported content type")
	}

	if b == nil {
		return nil, errors.New("no content")
	}

	data := new(UploadResult)
	err = f.upload(b, newParams("upload").Set("biz_attr", bizAttr).Set("sha", sha1sum(b)), data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (f File) upload(content []byte, body params, data interface{}) error {
	b := new(bytes.Buffer)
	w := multipart.NewWriter(b)

	for k, v := range body {
		if err := w.WriteField(k, v); err != nil {
			return err
		}
	}

	if content != nil {
		file, err := w.CreateFormFile("filecontent", "")
		if err != nil {
			return err
		}
		if _, err := file.Write(content); err != nil {
			return err
		}
	}

	if err := w.Close(); err != nil {
		return err
	}

	header := setHeader(newHeader(f.Sign(SignSeconds)), "multipart/form-data; boundary="+w.Boundary(), b.Len())
	return do("POST", f.dir.bucket.getURL(f.FullName()), header, b, data)
}

type uploadFirstPartResult struct {
	SliceSize int `json:"slice_size"`
	uploadPartResult
}

type uploadPartResult struct {
	Offset  int64  `json:"offset"`
	Session string `json:"session"`
	UploadResult
}

func zeroUploadPartResult(part *uploadPartResult) *uploadPartResult {
	if part != nil {
		part.AccessURL = ""
		part.Offset = 0
		part.ResourcePath = ""
		part.Session = ""
		part.SourceURL = ""
		part.URL = ""
	} else {
		part = new(uploadPartResult)
	}
	return part
}

// UploadSlice 分片上传本地文件，用于较大文件(一般大于8MB)
//  localFile 本地文件名
//  bizAttr 文件属性
//  sliceSize 设置分片上传大小，如果小于等于0则使用默认分片大小
//  session 用于断点续传；如果上传失败，则第二个string返回值为这次上传的session(可能为空)，下次传入该值进行断点续传
func (f File) UploadSlice(localFile, bizAttr string, sliceSize int, session string) (*UploadResult, string, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	size := stat.Size()

	h := sha1.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return nil, "", err
	}

	if sliceSize <= 0 {
		sliceSize = SliceSize
	}

	body := newParams("upload_slice").
		Set("biz_attr", bizAttr).
		Set("slice_size", strconv.Itoa(sliceSize)).
		Set("session", session).
		Set("sha", strings.ToUpper(hex.EncodeToString(h.Sum(nil)))).
		Set("filesize", strconv.FormatInt(size, 10))

	firstPart := new(uploadFirstPartResult)

	err = f.upload(nil, body, firstPart)
	if err != nil {
		return nil, "", err
	}

	if firstPart.URL != "" {
		return &firstPart.UploadResult, "", nil
	}

	if firstPart.Session != "" {
		session = firstPart.Session
	}
	body = newParams("upload_slice").Set("session", session)

	offset := firstPart.Offset
	_, err = file.Seek(offset, os.SEEK_SET)
	if err != nil {
		return nil, session, err
	}

	var part *uploadPartResult

	if firstPart.SliceSize > 0 {
		sliceSize = firstPart.SliceSize
	}
	b := make([]byte, sliceSize)

	for size > offset {
		n, err := file.Read(b)
		if err != nil && err != io.EOF {
			return nil, session, err
		}

		if n > 0 {
			part = zeroUploadPartResult(part)

			err = f.upload(b[:n], body.
				Set("sha", sha1sum(b[:n])).
				Set("offset", strconv.FormatInt(offset, 10)), part)
			if err != nil {
				return nil, session, err
			}

			if part.URL != "" {
				return &part.UploadResult, "", nil
			}

			if part.Session != session {
				return nil, "", errors.New("session corrupt")
			}

			offset += int64(n)
		}

		if err == io.EOF {
			break
		}
	}

	return nil, session, io.ErrUnexpectedEOF
}

// Update 更新文件信息
//  bizAttr 文件属性
func (f File) Update(bizAttr string) error {
	return f.post(f.Sign(0), newParams("update").Set("biz_attr", bizAttr), nil)
}

// PathInfo 表示目录信息或文件信息
type PathInfo struct {
	AccessURL string `json:"access_url"`
	BizAttr   string `json:"biz_attr"`
	Ctime     string `json:"ctime"`
	FileLen   int    `json:"filelen"`
	FileSize  int    `json:"filesize"`
	Mtime     string `json:"mtime"`
	Name      string `json:"name"`
	Sha       string `json:"sha"`
	SourceURL string `json:"source_url"`
}

// Stat 查询文件信息
func (f File) Stat() (*PathInfo, error) {
	m := make(map[string]string, 9)
	err := f.get(f.Sign(SignSeconds), newParams("stat"), &m)
	if err != nil {
		return nil, err
	}
	data := &PathInfo{
		AccessURL: m["access_url"],
		BizAttr:   m["biz_attr"],
		Ctime:     m["ctime"],
		Mtime:     m["mtime"],
		Name:      m["name"],
		Sha:       m["sha"],
		SourceURL: m["source_url"],
	}
	data.FileLen, err = strconv.Atoi(m["filelen"])
	if err != nil {
		return nil, err
	}
	data.FileSize, err = strconv.Atoi(m["filesize"])
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Delete 删除文件
func (f File) Delete() error {
	return f.post(f.Sign(0), newParams("delete"), nil)
}

// Name 返回已转义的文件名称，无首尾斜杠(/)
func (f File) Name() string {
	return EscapePath(f.name)
}

// FullName 返回已转义的目录名称加文件名称
func (f File) FullName() string {
	return f.dir.Name() + f.Name()
}

// Sign 根据seconds过期秒数返回文件的多次(大于0时)签名或单次(小于等于0时)签名
func (f File) Sign(seconds int) string {
	return f.dir.bucket.sign(f.FullName(), seconds)
}

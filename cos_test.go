// Copyright 2016 Chen Xianren. All rights reserved.

package cos

import (
	"os"
	"runtime"
	"testing"
)

func stack(t *testing.T) {
	b := make([]byte, 10*1024)
	t.Logf("%s", b[:runtime.Stack(b, false)])
}

func fatal(t *testing.T, err error) {
	if err != nil {
		stack(t)
		t.Fatal(err)
	}
}

func equal(t *testing.T, what string, expected, got interface{}) {
	if expected != got {
		stack(t)
		t.Fatal(what, "expected", expected, "but got", got)
	}
}

var testBucket = New(os.Getenv("COSAppID"), os.Getenv("COSSecretID"), os.Getenv("COSSecretKey")).
	Bucket(os.Getenv("COSBucket"))

func TestSign(t *testing.T) {
	b := New("200001", "AKIDUfLUEUigQiXqm7CVSspKJnuaiIKtxqAv", "bLcPnl88WU30VY57ipRhSePfPdOfSruK").
		Bucket("newbucket")

	const (
		more = "5bIObv9KXNcITrcVNRGCLG3K6xxhPTIwMDAwMSZiPW5ld2J1Y2tldCZrPUFLSURVZkxVRVVpZ1FpWHFtN0NWU3NwS0pudWFpSUt0eHFBdiZlPTE0Mzg2NjkxMTUmdD0xNDM2MDc3MTE1JnI9MTExNjImZj0="
		once = "OXy21aC6AjhScJaJqrBxcS0Y7lNhPTIwMDAwMSZiPW5ld2J1Y2tldCZrPUFLSURVZkxVRVVpZ1FpWHFtN0NWU3NwS0pudWFpSUt0eHFBdiZlPTAmdD0xNDM2MDc3MTE1JnI9MTExNjImZj0vMjAwMDAxL25ld2J1Y2tldC90ZW5jZW50X3Rlc3QuanBn"
	)

	got := b.cos.sign(b.name, 1438669115, 1436077115, 11162, "")
	equal(t, "多次签名", more, got)

	got = b.cos.sign(b.name, 0, 1436077115, 11162, "/200001/newbucket/tencent_test.jpg")
	equal(t, "单次签名", once, got)
}

const hi = "Hello world!"

func check(t *testing.T) {
	if testBucket.cos.appID == "" ||
		testBucket.cos.secretID == "" ||
		testBucket.cos.secretKey == "" ||
		testBucket.name == "" {
		t.SkipNow()
	}
}

func TestDir(t *testing.T) {
	check(t)
	d := testBucket.Dir("newdir")
	r, err := d.Create("")
	fatal(t, err)

	defer func() {
		fatal(t, d.Delete())
	}()

	fatal(t, d.Update(hi))

	info, err := d.Stat()
	fatal(t, err)

	equal(t, "创建时间", r.Ctime, info.Ctime)
	equal(t, "目录属性", hi, info.BizAttr)
	equal(t, "目录名称", d.name, info.Name)

	list, err := d.List(nil)
	fatal(t, err)
	equal(t, "目录数量", 0, list.DirCount)
	equal(t, "文件数量", 0, list.FileCount)
	equal(t, "目录和文件数量", 0, len(list.Infos))
}

func TestFile(t *testing.T) {
	check(t)
	f := testBucket.Dir("").File("newfile")

	content := []byte(hi)

	r, err := f.Upload(content, "")
	fatal(t, err)
	defer func() {
		fatal(t, f.Delete())
	}()

	info, err := f.Stat()
	fatal(t, err)

	equal(t, "文件访问链接", r.AccessURL, info.AccessURL)
	equal(t, "文件大小", len(content), info.FileSize)
	equal(t, "文件哈希", sha1sum(content), info.Sha)
	equal(t, "文件名称", f.name, info.Name)

	list, err := f.dir.List(nil)
	fatal(t, err)
	equal(t, "目录数量", 0, list.DirCount)
	equal(t, "文件数量", 1, list.FileCount)
	equal(t, "目录和文件数量", 1, len(list.Infos))
	equal(t, "文件哈希", info.Sha, list.Infos[0].Sha)

	if localFile := os.Getenv("COSUploadSlice"); localFile != "" {
		sliceFile := f.dir.File("newslicefile")
		r, _, err := sliceFile.UploadSlice(localFile, "", 0, "")
		fatal(t, err)
		defer func() {
			fatal(t, sliceFile.Delete())
		}()

		info, err := sliceFile.Stat()
		fatal(t, err)
		equal(t, "文件访问链接", r.AccessURL, info.AccessURL)
		equal(t, "文件名称", sliceFile.name, info.Name)

		list, err := sliceFile.dir.PrefixSearch("newslice", NewListDirParams().Num(10).Order(1))
		fatal(t, err)
		equal(t, "目录数量", 0, list.DirCount)
		equal(t, "文件数量", 1, list.FileCount)
		equal(t, "目录和文件数量", 1, len(list.Infos))
		equal(t, "文件哈希", info.Sha, list.Infos[0].Sha)
	} else {
		t.Skip("分片上传文件")
	}
}

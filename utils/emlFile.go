package utils

import (
	"bytes"
	ctx "context"
	"encoding/base64"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"unsafe"

	"github.com/minio/minio-go/v7"
	log "github.com/sirupsen/logrus"
	"github.com/uptrace/go-clickhouse/ch"
)

func GetEmlFilePath(dir string) []string {
	// 获取指定路径下的所有eml文件
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".eml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}
func ReadEml(emlPath string) string {
	content, err := ioutil.ReadFile(emlPath)
	if err != nil {
		panic(err)
	}
	return string(content)
}

func StringToBytes(data string) []byte {
	return *(*[]byte)(unsafe.Pointer(&data))
}

func BytesToString(data []byte) string {
	return string(data)
}

func GetClickHouseEmlFilePath(server string, port int, username string, password string, database string, startTime string, endTime string) []string {
	var emlFilePathList []string
	dsn := "clickhouse://" + username + ":" + password + "@" + server + ":" + strconv.Itoa(port) + "/" + database + "?sslmode=disable"
	db := ch.Connect(ch.WithDSN(dsn), ch.WithTimeout(5*time.Second), ch.WithDialTimeout(5*time.Second), ch.WithReadTimeout(5*time.Second), ch.WithWriteTimeout(5*time.Second), ch.WithPoolSize(100))
	results, err := db.Query("select emlFile from primitive_mail where ts >= '" + startTime + "' and ts <= '" + endTime + "'")

	if err != nil {
		return nil
	}
	for results.Next() {
		var emlFilePath *string
		err := results.Scan(&emlFilePath)
		if err != nil {
			panic(err)
		}

		emlFilePathList = append(emlFilePathList, decodeBase64(toString(emlFilePath)))
	}
	return emlFilePathList
}

func toString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func decodeBase64(data string) string {
	rawDecodedText, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Error(err)
	}
	return BytesToString(rawDecodedText)
}

func GetEmlFileForMinio(path string, minioClient *minio.Client) []byte {
	// 将path中的/eml替换为空
	path = path[5:]
	//log.Info(path)
	emlContent, err := minioClient.GetObject(ctx.Background(), "eml", path, minio.GetObjectOptions{})
	if err != nil {
		log.Error(err)
	}
	return StreamToByte(emlContent)
}

func StreamToByte(stream io.Reader) []byte {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(stream)
	if err != nil {
		return nil
	}
	return buf.Bytes()
}

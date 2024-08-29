package utils

import (
	"bytes"
	"os"

	log "github.com/sirupsen/logrus"
)

type Account struct {
	Username string
	Password string
}

func ReadAccountConfig(configPath string) []Account {
	//读取账号配置文件内容
	var accountConfig []Account
	content, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("无法读取文件: %v", err)
	}

	//按行读取文件内容
	lines := bytes.Split(content, []byte("\n"))
	for _, line := range lines {
		//根据,分割账号和密码
		account := bytes.Split(line, []byte(","))
		if len(account) != 2 {
			log.Error(line)
			log.Fatalf("账号配置文件格式错误")
		}
		//将账号和密码存入accountConfig
		accountConfig = append(accountConfig, Account{Username: string(account[0]), Password: string(account[1])})
	}
	return accountConfig
}

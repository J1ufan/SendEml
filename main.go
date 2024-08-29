package main

import (
	"crypto/tls"
	"net"
	"net/smtp"
	"os"
	"os/signal"
	"sendmail/utils"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	// 间隔时间单位 秒，毫秒，微秒，纳秒 映射到 time.Sleep() 的单位
	TIME_UNIT map[string]time.Duration = map[string]time.Duration{
		"s":  time.Second,
		"ms": time.Millisecond,
		"us": time.Microsecond,
		"ns": time.Nanosecond,
	}
)

func init() {
	// 设置日志格式为json格式
	log.SetFormatter(&log.JSONFormatter{})

	// 设置将日志输出到标准输出（默认的输出为stderr，标准错误）
	// 日志消息输出可以是任意的io.writer类型
	log.SetOutput(os.Stdout)

	// 设置日志级别为warn以上
	log.SetLevel(log.InfoLevel)
}

func main() {
	app := &cli.App{
		Name:    "SendMail",
		Usage:   "发送eml文件到邮件服务器",
		Version: "v1.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "from",
				Value: "from@example.com",
				Usage: "设置SMTP发件人",
			},
			&cli.StringFlag{
				Name:  "to",
				Value: "to@example.com",
				Usage: "设置SMTP收件人",
			},
			&cli.StringFlag{
				Name:  "server",
				Value: "127.0.0.1",
				Usage: "设置SMTP服务器地址",
			},
			&cli.IntFlag{
				Name:  "port",
				Value: 25,
				Usage: "设置SMTP服务器端口",
				Action: func(context *cli.Context, i int) error {
					if i >= 65535 {
						log.Info("端口必须小于65535")
					}
					return nil
				},
			},
			&cli.IntFlag{
				Name:  "sleep",
				Value: 0,
				Usage: "设置发件的间隔时间",
			},
			&cli.StringFlag{
				Name:  "sleepUnit",
				Value: "s",
				Usage: "设置发件的间隔时间单位 s,ms,us,ns",
				Action: func(context *cli.Context, s string) error {
					if _, ok := TIME_UNIT[s]; !ok {
						log.Info("sleepUnit必须为s,ms,us,ns")
					}
					return nil
				},
			},
			&cli.IntFlag{
				Name:  "timeThreshold",
				Value: 0,
				Usage: "设置发送邮件的时间阈值",
			},
			&cli.StringFlag{
				Name:  "accountConfig",
				Value: "",
				Usage: "指定账户信息文件",
			},

			&cli.IntFlag{
				Name:  "thread",
				Value: 1,
				Usage: "设置线程数",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "Anonymous",
				Usage:  "匿名发送eml文件",
				Action: anonymousSenderMode,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "dir",
						Value: "",
						Usage: "设置eml文件目录",
						Action: func(context *cli.Context, s string) error {
							// 判断s是否为路径
							_, err := os.Stat(s)
							if err != nil {
								log.Info("dir必须为目录")
								return err
							}
							return nil
						},
						Required: true,
					},
				},
			},
			{
				Name:   "Login",
				Usage:  "登录邮件服务器发送eml文件",
				Action: loginSenderMode,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "password",
						Value: "",
						Usage: "设置SMTP登录密码",
						Action: func(context *cli.Context, s string) error {
							if s == "" {
								log.Info("登录模式下密码不能为空")
							}
							return nil
						},
						// Required: true,
					},
					&cli.StringFlag{
						Name:  "dir",
						Value: "",
						Usage: "设置eml文件目录",
						Action: func(context *cli.Context, s string) error {
							// 判断s是否为路径
							_, err := os.Stat(s)
							if err != nil {
								log.Info("登录模式下dir必须为目录")
								return err
							}
							return nil
						},
						Required: true,
					},
				},
			},
			{
				Name:   "Replay",
				Usage:  "从minio中提取eml文件进行重放",
				Action: replaySenderMode,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "minio",
						Value: "127.0.0.1",
						Usage: "设置minio的IP地址",
					},
					&cli.IntFlag{
						Name:  "minioPort",
						Value: 7000,
						Usage: "设置minio的端口",
						Action: func(context *cli.Context, i int) error {
							if i >= 65535 {
								log.Info("端口必须小于65535")
							}
							return nil
						},
					},
					&cli.StringFlag{
						Name:  "minioUser",
						Value: "minioadmin",
						Usage: "设置minio用户名",
					},
					&cli.StringFlag{
						Name:  "minioPassword",
						Value: "minioadmin",
						Usage: "设置minio密码",
						Action: func(context *cli.Context, s string) error {
							if s == "" {
								log.Info("minio密码不能为空")
							}
							return nil
						},
					},
					&cli.StringFlag{
						Name:  "clickhouse",
						Value: "127.0.0.1",
						Usage: "设置clickhouse的IP地址",
					},
					&cli.IntFlag{
						Name:  "ckPort",
						Value: 9000,
						Usage: "设置clickhouse的端口",
						Action: func(context *cli.Context, i int) error {
							if i >= 65535 {
								log.Info("端口必须小于65535")
							}
							return nil
						},
					},
					&cli.StringFlag{
						Name:  "ckUser",
						Value: "default",
						Usage: "设置clickhouse的登录用户名",
					},
					&cli.StringFlag{
						Name:  "ckPassword",
						Value: "password",
						Usage: "设置clickhouse的登录密码",
					},
					&cli.StringFlag{
						Name:  "ckDatabase",
						Value: "default",
						Usage: "设置clickhouse中的数据库",
					},
					&cli.StringFlag{
						Name:  "startTime",
						Value: "2006-01-02 15:04:05",
						Usage: "设置clickhouse中数据的开始时间",
					},
					&cli.StringFlag{
						Name:  "endTime",
						Value: time.Now().Format("2006-01-02 15:04:05"),
						Usage: "设置clickhouse中数据的结束时间",
					},
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func anonymousSenderMode(context *cli.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	start := time.Now()
	senderNum := 0
	log.Info("Anonymous Sender Mode")
	emlFilePathList := utils.GetEmlFilePath(context.String("dir"))
	log.Info("获取到的eml文件数量：" + strconv.Itoa(len(emlFilePathList)) + "封")
	maxThreatCount := context.Int("thread")
	accountConfig := context.String("accountConfig")
	accountInfo := []utils.Account{}
	accountIndex := 0
	if accountConfig != "" {
		log.Info("账户信息文件为：" + accountConfig)
		// 读取账户信息文件
		accountInfo = utils.ReadAccountConfig(accountConfig)
		log.Info("账户信息为：", accountInfo)
	}
	// results := make(chan string, len(emlFilePathList))
	var wg sync.WaitGroup
	wg.Add(len(emlFilePathList))
	workerPool := make(chan struct{}, maxThreatCount)
	mailServer := context.String("server") + ":" + strconv.Itoa(context.Int("port"))
	sleepUnit := TIME_UNIT[context.String("sleepUnit")]
	var timeList []time.Duration
	// 判断是否设置了时间阈值
	log.Info("设置的时间阈值为：", context.Int("timeThreshold"))
	if context.Int("timeThreshold") > 0 {
		log.Info("设置了时间阈值，将在设定时间到达后退出程序")
		timer := time.NewTimer(time.Duration(context.Int("timeThreshold")) * time.Minute)
		// 创建一个数组用于保存时间戳
		// 遍历eml文件列表
		for _, emlFilePath := range emlFilePathList {
			select {
			case <-timer.C:
				log.Info("发送邮件设定时间到达，程序退出")
				goto END
			case <-sigChan:
				elapsed := time.Since(start)
				log.Info("程序退出")
				log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
				log.Info("发送邮件总耗时：", elapsed)
				log.Infof("发送邮件总数量：%s 封,读取到邮件总数为：%s", strconv.Itoa(senderNum), strconv.Itoa(len(emlFilePathList)))
				return nil
			default:
				senderNum++
				log.Infof("发送邮件：%s,第%s封", emlFilePath, strconv.Itoa(senderNum))
				SleepTime := context.Int("sleep")
				time.Sleep(time.Duration(SleepTime) * sleepUnit)
				workerPool <- struct{}{}
				go func(emlFilePath string) {
					// 读取eml文件转为字节
					defer func() {
						<-workerPool
						wg.Done()
					}()
					defer func() {
						if err := recover(); err != nil {
							log.Error(err)
						}
					}()
					emlContent := utils.StringToBytes(utils.ReadEml(emlFilePath))
					// 获取现在的时间
					now := time.Now()
					// 发送邮件
					tlsConfig := &tls.Config{
						InsecureSkipVerify: true,
						ServerName:         context.String("server"),
					}
					var conn net.Conn
					var err error
					if context.Int("port") == 25 {
						conn, err = net.Dial("tcp", mailServer)
						if err != nil {
							log.Println(err)
							return
						}
					} else {
						conn, err = tls.Dial("tcp", mailServer, tlsConfig)
						if err != nil {
							log.Println(err)
							return
						}
					}
					defer conn.Close()
					client, err := smtp.NewClient(conn, context.String("server"))
					if err != nil {
						log.Println(err)
						return
					}
					//if err = client.Auth(auth); err != nil {
					//	log.Printf("Auth err:%s", err)
					//	return
					//}
					if accountConfig != "" {
						log.Infof("发送邮件的账户为：%s", accountInfo)
						if accountIndex >= len(accountInfo) {
							accountIndex = 0
						}
						emailAddr := accountInfo[accountIndex].Username
						log.Infof("发送邮件的账户为：%s", emailAddr)
						if err := client.Mail(emailAddr); err != nil {
							log.Printf("Mail err:%s", err)
						}
						accountIndex++
					} else {
						if err := client.Mail(context.String("from")); err != nil {
							log.Printf("Mail err:%s", err)
						}
					}

					if err := client.Rcpt(context.String("to")); err != nil {
						log.Printf("rcpt err:%s", err)
					}

					writer, err := client.Data() // 获取写入器，用于写入邮件内容
					if err != nil {
						log.Printf("data err:%s", err)
					}

					_, err = writer.Write(emlContent) // 写入邮件内容
					if err != nil {
						log.Printf("write err:%s", err)
					}

					if err := writer.Close(); err != nil { // 关闭写入器
						log.Printf("close err:%s", err)
					}
					finishTime := time.Now()
					duration := finishTime.Sub(now)
					log.Infof("发送邮件：%s,耗时：%s", emlFilePath, duration)
					log.Info("发送邮件耗时：", duration)
					timeList = append(timeList, duration)
					// 计算发送邮件消耗的时间
					// results <- emlFilePath + " done"
				}(emlFilePath)

			}

		}
		wg.Wait()
	} else {
		log.Info("未设置时间阈值，将一直发送邮件,直到发送完毕")
		for _, emlFilePath := range emlFilePathList {
			select {
			case <-sigChan:
				elapsed := time.Since(start)
				log.Info("程序退出")
				log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
				log.Info("发送邮件总耗时：", elapsed)
				log.Infof("发送邮件总数量：%s 封,读取到邮件总数为：%s", strconv.Itoa(senderNum), strconv.Itoa(len(emlFilePathList)))
				return nil
			default:
				senderNum++
				log.Infof("发送邮件：%s,第%s封", emlFilePath, strconv.Itoa(senderNum))
				SleepTime := context.Int("sleep")
				time.Sleep(time.Duration(SleepTime) * sleepUnit)
				workerPool <- struct{}{}
				go func(emlFilePath string) {
					// 读取eml文件转为字节
					defer func() {
						<-workerPool
						wg.Done()
					}()
					defer func() {
						if err := recover(); err != nil {
							log.Error(err)
						}
					}()
					emlContent := utils.StringToBytes(utils.ReadEml(emlFilePath))
					// 获取现在的时间
					now := time.Now()
					// 发送邮件
					tlsConfig := &tls.Config{
						InsecureSkipVerify: true,
						ServerName:         context.String("server"),
					}
					var conn net.Conn
					var err error
					if context.Int("port") == 25 {
						conn, err = net.Dial("tcp", mailServer)
						if err != nil {
							log.Println(err)
							return
						}
					} else {
						conn, err = tls.Dial("tcp", mailServer, tlsConfig)
						if err != nil {
							log.Println(err)
							return
						}
					}
					defer conn.Close()
					client, err := smtp.NewClient(conn, context.String("server"))
					if err != nil {
						log.Println(err)
						return
					}
					//if err = client.Auth(auth); err != nil {
					//	log.Printf("Auth err:%s", err)
					//	return
					//}
					if accountConfig != "" {
						if accountIndex >= len(accountInfo) {
							accountIndex = 0
						}
						emailAddr := accountInfo[accountIndex].Username
						log.Infof("发送邮件的账户为：%s", emailAddr)
						if err := client.Mail(emailAddr); err != nil {
							log.Printf("Mail err:%s", err)
						}
						accountIndex++
					} else {
						if err := client.Mail(context.String("from")); err != nil {
							log.Printf("Mail err:%s", err)
						}
					}

					if err := client.Rcpt(context.String("to")); err != nil {
						log.Printf("rcpt err:%s", err)
					}

					writer, err := client.Data() // 获取写入器，用于写入邮件内容
					if err != nil {
						log.Printf("data err:%s", err)
					}

					_, err = writer.Write(emlContent) // 写入邮件内容
					if err != nil {
						log.Printf("write err:%s", err)
					}

					if err := writer.Close(); err != nil { // 关闭写入器
						log.Printf("close err:%s", err)
					}
					finishTime := time.Now()
					duration := finishTime.Sub(now)
					// log.Infof("发送邮件：%s,耗时：%s", emlFilePath, duration)
					// log.Info("发送邮件耗时：", duration)
					timeList = append(timeList, duration)
					// 计算发送邮件消耗的时间
					// results <- emlFilePath + " done"
				}(emlFilePath)

			}

		}
		wg.Wait()
	}
END:
	// close(results)
	elapsed := time.Since(start)
	// 计算发件的平均时间
	var sum time.Duration
	for _, v := range timeList {
		sum += v
	}
	avg := sum / time.Duration(len(timeList))
	log.Info("平均发送邮件耗时：", avg)
	// 获取最大的发件时间
	var max time.Duration
	for _, v := range timeList {
		if v > max {
			max = v
		}
	}
	log.Info("最大发送邮件耗时：", max)
	log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
	log.Info("发送邮件总耗时：", elapsed)
	log.Infof("发送邮件总数量：%s 封,读取到邮件总数为：%s", strconv.Itoa(senderNum), strconv.Itoa(len(emlFilePathList)))
	return nil
}

func loginSenderMode(context *cli.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	start := time.Now()
	senderNum := 0
	emlFilePathList := utils.GetEmlFilePath(context.String("dir"))
	log.Info("获取到的eml文件数量：" + strconv.Itoa(len(emlFilePathList)) + "封")
	maxThreatCount := context.Int("thread")
	results := make(chan string, len(emlFilePathList))
	var wg sync.WaitGroup
	wg.Add(len(emlFilePathList))
	workerPool := make(chan struct{}, maxThreatCount)
	sleepUnit := TIME_UNIT[context.String("sleepUnit")]
	var timeList []time.Duration
	accountConfig := context.String("accountConfig")
	accountInfo := []utils.Account{}
	accountIndex := 0
	if accountConfig != "" {
		log.Info("账户信息文件为：" + accountConfig)
		// 读取账户信息文件
		accountInfo = utils.ReadAccountConfig(accountConfig)
		log.Info("账户信息为：", accountInfo)
	}
	// 遍历eml文件列表
	if context.Int("timeThreshold") > 0 {
		log.Info("设置了时间阈值，将在设定时间到达后退出程序")
		timer := time.NewTimer(time.Duration(context.Int("timeThreshold")) * time.Minute)
		for _, emlFilePath := range emlFilePathList {
			select {
			case <-sigChan:
				elapsed := time.Since(start)
				log.Info("程序退出")
				log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
				log.Info("发送邮件总耗时：", elapsed)
				log.Infof("发送邮件总数量：%s 封", strconv.Itoa(senderNum))
				return nil
			case <-timer.C:
				log.Info("发送邮件设定时间到达，程序退出")
				goto END
			default:
				senderNum++
				log.Infof("发送邮件：%s,第%s封", emlFilePath, strconv.Itoa(senderNum))
				SleepTime := context.Int("sleep")
				time.Sleep(time.Duration(SleepTime) * sleepUnit)
				workerPool <- struct{}{}
				go func(emlFilePath string) {
					defer func() {
						emailAddr := context.String("from")
						passWord := context.String("password")
						if accountConfig != "" {
							if accountIndex >= len(accountInfo) {
								accountIndex = 0
							}
							emailAddr = accountInfo[accountIndex].Username
							passWord = accountInfo[accountIndex].Password
							accountIndex++
						}
						auth := smtp.PlainAuth("", emailAddr, passWord, context.String("server"))
						// 读取eml文件转为字节
						mailServer := context.String("server") + ":" + strconv.Itoa(context.Int("port"))
						emlContent := utils.StringToBytes(utils.ReadEml(emlFilePath))
						// 跳过tls证书验证
						now := time.Now()
						tlsConfig := &tls.Config{
							InsecureSkipVerify: true,
							ServerName:         context.String("server"),
						}
						var conn net.Conn
						var err error
						if context.Int("port") == 25 {
							conn, err = net.Dial("tcp", mailServer)
							if err != nil {
								log.Println(err)
								return
							}
						} else {
							conn, err = tls.Dial("tcp", mailServer, tlsConfig)
							if err != nil {
								log.Println(err)
								return
							}
						}
						defer conn.Close()
						client, err := smtp.NewClient(conn, context.String("server"))
						if err != nil {
							log.Println(err)
							return
						}
						if err = client.Auth(auth); err != nil {
							log.Fatalf("Auth err:%s", err)
							return
						}
						if err := client.Mail(emailAddr); err != nil {
							log.Fatalf("Mail err:%s", err)
						}

						if err := client.Rcpt(context.String("to")); err != nil {
							log.Fatalf("rcpt err:%s", err)
						}

						writer, err := client.Data() // 获取写入器，用于写入邮件内容
						if err != nil {
							log.Printf("data err:%s", err)
						}

						_, err = writer.Write(emlContent) // 写入邮件内容
						if err != nil {
							log.Printf("write err:%s", err)
						}

						if err := writer.Close(); err != nil { // 关闭写入器
							log.Printf("close err:%s", err)
						}

						// 发送邮件
						// err := smtp.SendMail(mailServer, auth, context.String("from"), []string{context.String("to")}, emlContent)
						if err != nil {
							<-workerPool
							wg.Done()
							log.Error(err)
						}
						<-workerPool
						wg.Done()
						finishTime := time.Now()
						duration := finishTime.Sub(now)
						log.Infof("发送邮件：%s,耗时：%s", emlFilePath, duration)
						log.Info("发送邮件耗时：", duration)
						timeList = append(timeList, duration)
					}()
					results <- emlFilePath + " done"
				}(emlFilePath)
			}
		}
		wg.Wait()
	} else {
		log.Info("未设置时间阈值，将一直发送邮件,直到发送完毕")
		for _, emlFilePath := range emlFilePathList {
			select {
			case <-sigChan:
				elapsed := time.Since(start)
				log.Info("程序退出")
				log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
				log.Info("发送邮件总耗时：", elapsed)
				log.Infof("发送邮件总数量：%s 封", strconv.Itoa(senderNum))
				return nil
			default:
				senderNum++
				log.Infof("发送邮件：%s,第%s封", emlFilePath, strconv.Itoa(senderNum))
				SleepTime := context.Int("sleep")
				time.Sleep(time.Duration(SleepTime) * sleepUnit)
				workerPool <- struct{}{}
				go func(emlFilePath string) {
					defer func() {
						emailAddr := context.String("from")
						passWord := context.String("password")
						if accountConfig != "" {
							if accountIndex >= len(accountInfo) {
								accountIndex = 0
							}
							emailAddr = accountInfo[accountIndex].Username
							passWord = accountInfo[accountIndex].Password
							accountIndex++
						}
						auth := smtp.PlainAuth("", emailAddr, passWord, context.String("server"))
						// 读取eml文件转为字节
						mailServer := context.String("server") + ":" + strconv.Itoa(context.Int("port"))
						emlContent := utils.StringToBytes(utils.ReadEml(emlFilePath))
						// 跳过tls证书验证
						now := time.Now()
						tlsConfig := &tls.Config{
							InsecureSkipVerify: true,
							ServerName:         context.String("server"),
						}
						var conn net.Conn
						var err error
						if context.Int("port") == 25 {
							conn, err = net.Dial("tcp", mailServer)
							if err != nil {
								log.Error(err)
								return
							}
						} else {
							conn, err = tls.Dial("tcp", mailServer, tlsConfig)
							if err != nil {
								log.Error(err)
								return
							}
						}

						defer conn.Close()
						client, err := smtp.NewClient(conn, context.String("server"))
						if err != nil {
							log.Error(err)
							return
						}
						// 如果报错需要注释 Auth方法中的代码内容
						// if !server.TLS && !isLocalhost(server.Name) {
						// 	return "", nil, errors.New("unencrypted connection")
						// }
						if err = client.Auth(auth); err != nil {
							log.Fatalf("Auth err:%s", err)
						}
						if err := client.Mail(emailAddr); err != nil {
							log.Printf("Mail err:%s", err)
						}

						if err := client.Rcpt(context.String("to")); err != nil {
							log.Printf("rcpt err:%s", err)
						}

						writer, err := client.Data() // 获取写入器，用于写入邮件内容
						if err != nil {
							log.Printf("data err:%s", err)
						}

						_, err = writer.Write(emlContent) // 写入邮件内容
						if err != nil {
							log.Printf("write err:%s", err)
						}

						if err := writer.Close(); err != nil { // 关闭写入器
							log.Printf("close err:%s", err)
						}

						// 发送邮件
						// err := smtp.SendMail(mailServer, auth, context.String("from"), []string{context.String("to")}, emlContent)
						if err != nil {
							<-workerPool
							wg.Done()
							log.Error(err)
						}
						<-workerPool
						wg.Done()
						results <- emlFilePath + " done"
						finishTime := time.Now()
						duration := finishTime.Sub(now)
						log.Infof("发送邮件：%s,耗时：%s", emlFilePath, duration)
						log.Info("发送邮件耗时：", duration)
						timeList = append(timeList, duration)
					}()
				}(emlFilePath)
			}
		}
		wg.Wait()
	}
END:
	// close(results)
	close(results)
	elapsed := time.Since(start)
	// 计算发件的平均时间
	var sum time.Duration
	for _, v := range timeList {
		sum += v
	}
	avg := sum / time.Duration(len(timeList))
	log.Info("平均发送邮件耗时：", avg)
	// 获取最大的发件时间
	var max time.Duration
	for _, v := range timeList {
		if v > max {
			max = v
		}
	}
	log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
	log.Info("发送邮件总耗时：", elapsed)
	log.Infof("发送邮件总数量：%s 封,读取到邮件总数为：%s", strconv.Itoa(senderNum), strconv.Itoa(len(emlFilePathList)))
	return nil
}

func replaySenderMode(context *cli.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	start := time.Now()
	senderNum := 0
	log.Info("Replay Sender Mode")
	// 从clickhouse中读取eml文件路径
	emlFilePathList := utils.GetClickHouseEmlFilePath(context.String("clickhouse"), context.Int("ckPort"), context.String("ckUser"), context.String("ckPassword"), context.String("ckDatabase"), context.String("startTime"), context.String("endTime"))
	log.Info("获取到的eml文件数量：" + strconv.Itoa(len(emlFilePathList)) + "封")
	// 从minio中读取eml文件内容
	// 获取minio client
	minioClient, err := minio.New(context.String("minio")+":"+strconv.Itoa(context.Int("minioPort")), &minio.Options{
		Creds:  credentials.NewStaticV4(context.String("minioUser"), context.String("minioPassword"), ""),
		Secure: false,
	})
	if err != nil {
		log.Error(err)
		return err
	}
	maxThreatCount := context.Int("thread")
	results := make(chan string, len(emlFilePathList))
	var wg sync.WaitGroup
	wg.Add(len(emlFilePathList))
	workerPool := make(chan struct{}, maxThreatCount)

	// 遍历eml文件列表
	for _, emlFilePath := range emlFilePathList {
		select {
		case <-sigChan:
			elapsed := time.Since(start)
			log.Info("程序退出")
			log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
			log.Info("发送邮件总耗时：", elapsed)
			log.Infof("发送邮件总数量：%s 封,读取到邮件总数为：%s", strconv.Itoa(senderNum), strconv.Itoa(len(emlFilePathList)))
			return nil
		default:
			senderNum++
			log.Infof("发送邮件：%s,第%s封", emlFilePath, strconv.Itoa(senderNum))
			SleepTime := context.Int("sleep")
			time.Sleep(time.Duration(SleepTime) * time.Second)
			workerPool <- struct{}{}
			go func(emlFilePath string) {
				defer func() {
					// 读取eml文件转为字节
					emlContent := utils.GetEmlFileForMinio(emlFilePath, minioClient)
					// 输出读取到的eml文件内容
					//log.Info(emlContent)
					// 发送邮件
					err := smtp.SendMail(context.String("server")+":"+strconv.Itoa(context.Int("port")), nil, context.String("from"), []string{context.String("to")}, emlContent)
					if err != nil {
						<-workerPool
						wg.Done()
						log.Error(err)
					}
					<-workerPool
					wg.Done()
				}()
				results <- emlFilePath + " done"
			}(emlFilePath)
		}
	}
	wg.Wait()
	close(results)

	elapsed := time.Since(start)
	log.Info("开始发送邮件时间：", start.Format("2006-01-02 15:04:05"))
	log.Info("发送邮件总耗时：", elapsed)
	log.Infof("发送邮件总数量：%s 封,读取到邮件总数为：%s", strconv.Itoa(senderNum), strconv.Itoa(len(emlFilePathList)))
	return nil
}

# SendEml
使用GO语言编写的发件程序用于读取 eml 文件作为邮件内容发送到邮件服务器
# 使用方式
```
NAME:
   SendMail - 发送eml文件到邮件服务器

USAGE:
   SendMail [global options] command [command options] [arguments...]

VERSION:
   v1.0

COMMANDS:
   Anonymous  匿名发送eml文件
   Login      登录邮件服务器发送eml文件
   Replay     从minio中提取eml文件进行重放
   help, h    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --from value           设置SMTP发件人 (default: "from@example.com")
   --to value             设置SMTP收件人 (default: "to@example.com")
   --server value         设置SMTP服务器地址 (default: "127.0.0.1")
   --port value           设置SMTP服务器端口 (default: 25)
   --sleep value          设置发件的间隔时间 (default: 0)
   --sleepUnit value      设置发件的间隔时间单位 s,ms,us,ns (default: "s")
   --timeThreshold value  设置发送邮件的时间阈值 (default: 0)
   --accountConfig value  指定账户信息文件
   --thread value         设置线程数 (default: 1)
   --help, -h             show help
   --version, -v          print the version
```

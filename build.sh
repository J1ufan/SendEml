GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -ldflags "-w -s" -trimpath -o SendMail_linux_amd64
GOARCH=arm64 GOOS=darwin CGO_ENABLED=0 go build -ldflags "-w -s" -trimpath -o SendMail_darwin_arm64
GOARCH=amd64 GOOS=windows CGO_ENABLED=0 go build -ldflags "-w -s" -trimpath -o SendMail_windows_amd64.exe
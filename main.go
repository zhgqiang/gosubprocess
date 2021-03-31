package main

import (
	"bufio"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/lestrrat-go/file-rotatelogs"
	log "github.com/sirupsen/logrus"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func init() {
	writer, err := rotatelogs.New(
		"log"+".%Y%m%d%H%M",
		rotatelogs.WithLinkName("log"),           // 生成软链，指向最新日志文件
		rotatelogs.WithMaxAge(time.Hour*24*2),    // 文件最大保存时间
		rotatelogs.WithRotationTime(time.Hour*1), // 日志切割时间间隔
	)
	if err != nil {
		log.Fatal("Init log failed, err:", err)
	}
	log.SetOutput(writer)
	log.SetLevel(log.InfoLevel)
}

func main() {
	if len(os.Args) == 1 {
		log.Fatal("无可执行的命令参数")
	}
	if err := Run(func() {
		go func() {
			for {
				ch, err := getOutputContinually(os.Args[1], os.Args[2:]...)
				if err != nil {
					log.Println("执行程序错误,", err.Error())
				} else {
					<-ch
				}
				time.Sleep(time.Second * 10)
			}
		}()

	}); err != nil {
		log.Println("守护程序运行错误,", err.Error())
	}

}

func getOutputContinually(name string, args ...string) (<-chan struct{}, error) {
	cmd := exec.Command(name, args...)

	closed := make(chan struct{})
	defer close(closed)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdoutPipe.Close()

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() { // 命令在执行的过程中, 实时地获取其输出
			switch runtime.GOOS {
			case "darwin":
				log.Println(string(scanner.Bytes()))
			case "windows":
				data, err := simplifiedchinese.GB18030.NewDecoder().Bytes(scanner.Bytes()) // 防止乱码
				if err != nil {
					log.Println("transfer error with bytes:", scanner.Bytes())
					continue
				}
				log.Println(string(data))
			case "linux":
				log.Println(string(scanner.Bytes()))
			}

		}
	}()

	if err := cmd.Run(); err != nil {
		return nil, err
	}
	//log.Println("进程ID: ",cmd.ProcessState.Pid())
	return closed, nil
}

// Run 运行服务
func Run(run func()) error {
	state := 1
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	run()
EXIT:
	for {
		sig := <-sc
		switch sig {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			state = 0
			break EXIT
		case syscall.SIGHUP:
		default:
			break EXIT
		}
	}

	log.Println("服务退出")
	time.Sleep(time.Second)
	os.Exit(state)
	return nil
}

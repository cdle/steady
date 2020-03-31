package steady

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	//sh -c 从一个字符串中而不是从一个文件中读取并执行shell命令。
	sh = "sh"
	re = "-c"
)

var (
	processName string
	execPath    string
	processID   string
)

//init 初始化
func init() {
	if runtime.GOOS == "windows" {
		return
	}
	var err error
	//获取路径
	execPath, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logln("获取程序路径失败：", err)
	}
	//获取进程名
	ls := strings.Split(os.Args[0], "/")
	processName = ls[len(ls)-1]
	//获取进程ID
	processID = fmt.Sprint(os.Getpid())
	//检查指令
	if len(os.Args) > 1 {
		switch os.Args[1] {
		//热更新
		case "-graceful":
			logln("系统重启！")
			return
		case "-reload": //重新加载
			if err := Reload(); err != nil {
				logln("重新加载失败：", err)
				os.Exit(1)
			}
			logln("重新加载成功!")
			os.Exit(0)
		case "-update": //升级
			if err := UpdateProgram(); err != nil {
				logln("升级未完成：", err)
				os.Exit(1)
			}
			logln("升级成功！")
			os.Exit(0)
		case "-stop": //停止
			if err := Stop(); err != nil {
				logln("停止程序失败：", err)
				os.Exit(1)
			}
			logln("停止程序成功!")
			os.Exit(0)
		case "-tail": //查看日志，但会影响热重启，如果解决这个问题需要改变进程名称
			cmd := exec.Command(sh, re, "tail -f "+execPath+"/"+processName+".out")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logln("查看日志失败：", err)
			}
			cmd.Start()
			reader := bufio.NewReader(stdout)
			for {
				line, err2 := reader.ReadString('\n')
				if err2 != nil || io.EOF == err2 {
					break
				}
				fmt.Print(line)
			}
		case "-build": //编译、重启
			if err := CompileProgram(); err != nil {
				logln("代码编译失败：", err.Error())
				os.Exit(1)
			}
			logln("编译完成！")
			Reload()
			os.Exit(0)
		}
	}
	pids, _ := peersID()
	if len(pids) != 0 {
		logln("程序已运行" + fmt.Sprint(pids) + "!")
		os.Exit(1)
	}
	if os.Args[0] == "./"+processName {
		return
	}
	logln("尝试指令查看运行日志", "tail -f "+execPath+"/"+processName+".out")
	if err := StartProgram(); err != nil {
		logln("运行失败：", err)
		os.Exit(1)
	}
	os.Exit(0)
}

//cycGitPull 周期性更新代码
// func cycGitPull() {
// 	if runtime.GOOS == "darwin" {
// 		return
// 	}
// 	tiker := time.NewTicker(time.Hour)
// 	for {
// 		if err := GitPull(); err != nil {
// 			logln("周期性检查代码：", err)
// 			continue
// 		}
// 		if err := CompileProgram(); err != nil {
// 			logln("代码编译失败：", err)
// 			continue
// 		}
// 		InnerReload()
// 		<-tiker.C
// 	}
// }

//CompileProgram 编译程序
func CompileProgram() error {
	cmdStr := "cd " + execPath + " && go build -o " + processName
	_, err := exec.Command(sh, re, cmdStr).Output()
	if err != nil {
		return err
	}
	return nil
}

//GitPull 获取新代码
func GitPull() error {
	cmdStr := "cd " + execPath + " && git checkout . && git pull" //&& git checkout .
	rtn, err := exec.Command(sh, re, cmdStr).Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(rtn), "changed") {
		return errors.New(strings.TrimRight(string(rtn), "\n"))
	}
	return nil
}

//peersID 同伴ID
func peersID() ([]string, error) {
	pids := []string{}
	rtn, err := exec.Command(sh, re, "pidof "+processName).Output()
	if err != nil {
		return pids, err
	}
	re := regexp.MustCompile(`[\d]+`)
	for _, v := range re.FindAll(rtn, -1) {
		if string(v) != processID {
			pids = append(pids, string(v))
		}
	}
	return pids, nil
}

//InnerReload 内部重启程序
func InnerReload() error {
	pids, err := peersID()
	if err != nil {
		return errors.New("获取运行中程序：" + err.Error())
	}
	if len(pids) >= 1 {
		return errors.New("程序已在重启中！")
	}
	return exec.Command(sh, re, "kill -HUP "+processID).Start()
}

//Reload 重启程序
func Reload() error {
	pids, err := peersID()
	if err != nil {
		return errors.New("获取运行中程序：" + err.Error())
	}
	if len(pids) == 0 {
		return errors.New("程序未运行！")
	}
	if len(pids) >= 2 {
		return errors.New("程序已在重启中！")
	}
	return exec.Command(sh, re, "kill -HUP "+strings.Join(pids, " ")).Start()
}

//InnerStop 内部停止程序
func InnerStop() error {
	return exec.Command(sh, re, "kill "+processID).Start()
}

//Stop 停止程序
func Stop() error {
	pids, err := peersID()
	if err != nil {
		return errors.New("获取运行中程序：" + err.Error())
	}
	if len(pids) == 0 {
		return errors.New("程序未运行！")
	}
	return exec.Command(sh, re, "kill "+strings.Join(pids, " ")).Start()
}

//Fork 拉起新进程
func Fork() error {
	args := []string{}
	for _, arg := range os.Args {
		args = append(args, arg)
	}
	err := exec.Command(os.Args[0], args...).Start()
	if err != nil {
		return err
	}
	return nil
}

//UpdateProgram 更新程序
func UpdateProgram() error {
	if err := GitPull(); err != nil {
		return err
	}
	if err := CompileProgram(); err != nil {
		return err
	}
	Reload()
	return nil
}

//StartProgram 运行程序并指定输出位置
func StartProgram() error {
	cmdStr := "cd " + execPath + " && ./" + processName + " >> " + processName + ".out &"
	return exec.Command(sh, re, cmdStr).Start()
}

//logln 打印
func logln(args ...interface{}) {
	args = append([]interface{}{
		time.Now().Local().Format("2006/01/02 15:04:05"),
		processID,
	}, args...)
	fmt.Println(args...)
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// "d" (日) 単位を Go の time.Duration が理解できる "h" (時) に変換する
func parseDurationExtended(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([a-zA-Z]+)$`)
	matches := re.FindStringSubmatch(s)

	if len(matches) == 3 {
		value, _ := strconv.Atoi(matches[1])
		unit := strings.ToLower(matches[2])
		if unit == "d" {
			s = fmt.Sprintf("%dh", value*24)
		}
	}
	return time.ParseDuration(s)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mtimeout <duration> <command> [args...]")
		fmt.Println("Example: mtimeout 500ms sleep 1")
		fmt.Println("Units: ms, s, m, h, d")
		os.Exit(1)
	}

	// 1. タイムアウト時間のパース
	durationStr := os.Args[1]
	timeout, err := parseDurationExtended(durationStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid duration %q: %v\n", durationStr, err)
		os.Exit(1)
	}

	// 2. コマンドの準備
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdName := os.Args[2]
	cmdArgs := os.Args[3:]
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	// プロセスグループを新しく作成する設定を追加
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 標準入出力を現在のプロセスに繋ぐ
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// 3. 実行
	err = cmd.Run()

	// 4. 結果判定
	if ctx.Err() == context.DeadlineExceeded {
		// 子プロセスのプロセスグループID（PGID）に対して
		// マイナスの値を指定して Kill を送ると、グループ全体に届く
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		fmt.Fprintf(os.Stderr, "mtimeout: command timed out after %s\n", timeout)
		os.Exit(124) // gtimeoutの慣習に従い124で終了
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

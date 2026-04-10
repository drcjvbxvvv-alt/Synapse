package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// executeKubectlCommand 執行kubectl命令
func (h *KubectlTerminalHandler) executeKubectlCommand(session *KubectlSession, kubeconfigPath, namespace, command string) {
	// 解析命令
	parts := strings.Fields(command)
	if len(parts) == 0 {
		h.sendMessage(session.Conn, "command_result", "")
		return
	}

	// 處理特殊命令
	if h.handleSpecialCommands(session, command) {
		return
	}

	// 構建kubectl命令
	var args []string
	if parts[0] == "kubectl" {
		args = parts[1:]
	} else {
		// 如果使用者沒有輸入kubectl字首，自動新增
		args = parts
	}

	// 檢查是否需要新增namespace參數
	needsNamespace := h.commandNeedsNamespace(args)

	// 新增kubeconfig參數
	kubectlArgs := []string{"--kubeconfig", kubeconfigPath}

	// 如果命令需要namespace且使用者沒有指定，則新增預設namespace
	if needsNamespace && !h.hasNamespaceFlag(args) {
		kubectlArgs = append(kubectlArgs, "--namespace", namespace)
	}

	kubectlArgs = append(kubectlArgs, args...)

	// 檢查是否是流式命令（如 logs -f）
	isStreamingCommand := h.isStreamingCommand(args)

	// 建立命令
	var ctx context.Context
	var cancel context.CancelFunc

	if isStreamingCommand {
		// 流式命令不設定超時
		ctx, cancel = context.WithCancel(session.Context)
	} else {
		// 非流式命令設定超時
		ctx, cancel = context.WithTimeout(session.Context, 60*time.Second)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", kubectlArgs...) // #nosec G204 -- kubectl 參數經過白名單校驗

	// 設定環境變數
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath),
	)

	// 儲存命令到會話，以便可以被中斷
	session.Mutex.Lock()
	session.Cmd = cmd
	session.Mutex.Unlock()

	// 如果是流式命令，使用管道處理輸出
	if isStreamingCommand {
		// 建立管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("建立輸出管道失敗: %v", err))
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("建立錯誤管道失敗: %v", err))
			return
		}

		// 啟動命令
		if err := cmd.Start(); err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("啟動命令失敗: %v", err))
			return
		}

		// 讀取標準輸出
		go func() {
			buffer := make([]byte, wsBufferSize)
			for {
				n, err := stdout.Read(buffer)
				if n > 0 {
					h.sendMessage(session.Conn, "output", string(buffer[:n]))
				}
				if err != nil {
					break
				}
			}
		}()

		// 讀取標準錯誤
		go func() {
			buffer := make([]byte, wsBufferSize)
			for {
				n, err := stderr.Read(buffer)
				if n > 0 {
					h.sendMessage(session.Conn, "error", string(buffer[:n]))
				}
				if err != nil {
					break
				}
			}
		}()

		// 等待命令完成
		go func() {
			err := cmd.Wait()
			if err != nil && ctx.Err() != context.Canceled {
				h.sendMessage(session.Conn, "error", fmt.Sprintf("命令執行失敗: %v", err))
			}
			h.sendMessage(session.Conn, "command_result", "")

			// 清除會話中的命令引用
			session.Mutex.Lock()
			session.Cmd = nil
			session.Mutex.Unlock()
		}()
	} else {
		// 非流式命令，使用CombinedOutput
		output, err := cmd.CombinedOutput()

		// 清除會話中的命令引用
		session.Mutex.Lock()
		session.Cmd = nil
		session.Mutex.Unlock()

		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				h.sendMessage(session.Conn, "error", "命令執行超時 (60秒)")
			} else {
				h.sendMessage(session.Conn, "error", fmt.Sprintf("命令執行失敗: %v\n%s", err, string(output)))
			}
		} else {
			// 傳送輸出
			if len(output) > 0 {
				h.sendMessage(session.Conn, "output", string(output))
			}
		}

		h.sendMessage(session.Conn, "command_result", "")
	}
}

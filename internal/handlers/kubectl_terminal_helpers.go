package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// isStreamingCommand 檢查是否是流式命令
func (h *KubectlTerminalHandler) isStreamingCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	// 檢查是否是 logs 命令 (無論是否有 -f 參數，都作為流式處理)
	if args[0] == "logs" {
		return true
	}

	// 檢查是否是 exec 命令
	if args[0] == "exec" {
		return true
	}

	// 檢查是否是 port-forward 命令
	if args[0] == "port-forward" {
		return true
	}

	// 檢查是否是 watch 命令
	if args[0] == "watch" {
		return true
	}

	// 檢查是否是 top 命令
	if args[0] == "top" {
		return true
	}

	// 檢查命令列中是否包含 --watch 參數
	for _, arg := range args {
		if arg == "--watch" || arg == "-w" {
			return true
		}
	}

	return false
}

// handleSpecialCommands 處理特殊命令
func (h *KubectlTerminalHandler) handleSpecialCommands(session *KubectlSession, command string) bool {
	command = strings.TrimSpace(command)

	switch {
	case command == "clear" || command == "cls":
		h.sendMessage(session.Conn, "clear", "")
		h.sendMessage(session.Conn, "command_result", "")
		return true
	case command == "help" || command == "?":
		h.sendHelpMessage(session)
		return true
	case command == "history":
		h.sendHistoryMessage(session)
		return true
	case strings.HasPrefix(command, "ns "):
		// 切換namespace的快捷命令
		namespace := strings.TrimSpace(command[3:])
		if namespace != "" {
			session.Namespace = namespace
			h.sendMessage(session.Conn, "namespace_changed", namespace)
		}
		h.sendMessage(session.Conn, "command_result", "")
		return true
	}

	return false
}

// commandNeedsNamespace 檢查命令是否需要namespace
func (h *KubectlTerminalHandler) commandNeedsNamespace(args []string) bool {
	if len(args) == 0 {
		return false
	}

	// 不需要namespace的命令
	clusterCommands := []string{
		"cluster-info", "version", "api-versions", "api-resources",
		"get nodes", "get namespaces", "get ns", "get pv", "get sc",
		"get clusterroles", "get clusterrolebindings",
	}

	command := strings.Join(args, " ")
	for _, cmd := range clusterCommands {
		if strings.HasPrefix(command, cmd) {
			return false
		}
	}

	return true
}

// hasNamespaceFlag 檢查命令是否已經包含namespace參數
func (h *KubectlTerminalHandler) hasNamespaceFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-n" || arg == "--namespace" {
			return true
		}
		if strings.HasPrefix(arg, "--namespace=") {
			return true
		}
		if arg == "--all-namespaces" || arg == "-A" {
			return true
		}
	}
	return false
}

// sendHelpMessage 傳送幫助資訊
func (h *KubectlTerminalHandler) sendHelpMessage(session *KubectlSession) {
	helpText := `
kubectl終端幫助資訊:

基本命令:
  kubectl get pods              - 檢視Pod列表
  kubectl get nodes             - 檢視節點列表
  kubectl get svc               - 檢視服務列表
  kubectl get deployments      - 檢視部署列表
  kubectl describe pod <name>   - 檢視Pod詳情
  kubectl logs <pod-name>       - 檢視Pod日誌
  kubectl exec -it <pod> bash   - 進入Pod容器

快捷命令:
  clear/cls                     - 清屏
  help/?                        - 顯示幫助
  history                       - 顯示命令歷史
  ns <namespace>                - 切換命名空間

提示:
  - 可以省略kubectl字首，系統會自動新增
  - 使用Tab鍵可以自動補全(部分支援)
  - 使用上下箭頭鍵瀏覽歷史命令
  - 當前命名空間會自動應用到相關命令

`
	h.sendMessage(session.Conn, "output", helpText)
	h.sendMessage(session.Conn, "command_result", "")
}

// sendHistoryMessage 傳送歷史命令
func (h *KubectlTerminalHandler) sendHistoryMessage(session *KubectlSession) {
	if len(session.History) == 0 {
		h.sendMessage(session.Conn, "output", "暫無命令歷史\n")
	} else {
		historyText := "命令歷史:\n"
		for i, cmd := range session.History {
			historyText += fmt.Sprintf("  %d: %s\n", i+1, cmd)
		}
		h.sendMessage(session.Conn, "output", historyText)
	}
	h.sendMessage(session.Conn, "command_result", "")
}

// handleInterrupt 處理中斷訊號
func (h *KubectlTerminalHandler) handleInterrupt(session *KubectlSession) {
	session.Mutex.Lock()
	cmd := session.Cmd
	session.LastCommand = ""
	session.Mutex.Unlock()

	// 傳送中斷訊號到終端
	h.sendMessage(session.Conn, "output", "^C\n")

	// 如果有正在執行的命令，嘗試終止它
	if cmd != nil && cmd.Process != nil {
		logger.Info("正在終止命令", "pid", cmd.Process.Pid)

		// 在Windows上，Kill()可能不會立即終止程序，嘗試使用taskkill
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run() // #nosec G204 -- PID 來自已知程序
		} else {
			// 在Unix系統上，傳送SIGINT訊號（等同於Ctrl+C）
			_ = cmd.Process.Signal(syscall.SIGINT)

			// 給程序一點時間響應SIGINT
			time.Sleep(100 * time.Millisecond)

			// 如果程序仍在執行，強制終止
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				_ = cmd.Process.Kill()
			}
		}
	}

	h.sendMessage(session.Conn, "command_result", "")
}

// createTempKubeconfig 建立臨時kubeconfig檔案
func (h *KubectlTerminalHandler) createTempKubeconfig(cluster *models.Cluster) (string, error) {
	// 建立臨時檔案
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("建立臨時檔案失敗: %v", err)
	}
	defer func() {
		_ = tmpFile.Close()
	}()

	// 寫入kubeconfig內容
	var kubeconfigContent string
	if cluster.KubeconfigEnc != "" {
		kubeconfigContent = cluster.KubeconfigEnc
	} else if cluster.SATokenEnc != "" {
		// 從Token建立kubeconfig
		kubeconfigContent = services.CreateKubeconfigFromToken(
			cluster.Name,
			cluster.APIServer,
			cluster.SATokenEnc,
			cluster.CAEnc,
		)
	} else {
		return "", fmt.Errorf("叢集缺少認證資訊")
	}

	_, err = tmpFile.WriteString(kubeconfigContent)
	if err != nil {
		return "", fmt.Errorf("寫入kubeconfig失敗: %v", err)
	}

	return tmpFile.Name(), nil
}

// sendMessage 傳送WebSocket訊息
func (h *KubectlTerminalHandler) sendMessage(conn *websocket.Conn, msgType, data string) {
	msg := TerminalMessage{
		Type: msgType,
		Data: data,
	}

	if err := conn.WriteJSON(msg); err != nil {
		logger.Error("傳送WebSocket訊息失敗", "error", err)
	}
}

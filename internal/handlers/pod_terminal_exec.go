package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// findAvailableShell 查詢可用的shell
// 策略：直接以絕對路徑執行各 shell，不依賴 sh 包裝（極簡容器可能連 sh 都沒有）
func (h *PodTerminalHandler) findAvailableShell(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession) (string, error) {
	// 按常見程度排序，優先嘗試完整路徑，再嘗試裸名（PATH 查找）
	candidates := []string{
		"/bin/bash", "/usr/bin/bash",
		"/bin/sh", "/usr/bin/sh",
		"/bin/ash", "/usr/bin/ash",
		"/bin/dash", "/usr/bin/dash",
		"/bin/zsh", "/usr/bin/zsh",
		"/bin/ksh", "/usr/bin/ksh",
	}

	for _, shell := range candidates {
		if h.tryExecShell(client, k8sConfig, session, shell) {
			return shell, nil
		}
	}

	return "", fmt.Errorf("未找到任何可用的shell")
}

// tryExecShell 直接執行指定 shell 路徑，成功回傳 true
// 不使用 sh -c 包裝，避免容器內無 sh 時全部誤判為不存在
func (h *PodTerminalHandler) tryExecShell(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession, shell string) bool {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(session.PodName).
		Namespace(session.Namespace).SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: session.Container,
		Command:   []string{shell, "-c", "echo ok"},
		Stdout:    true,
		Stderr:    false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k8sConfig, "POST", req.URL())
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var buf bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &buf, Tty: false})
	return err == nil && strings.TrimSpace(buf.String()) == "ok"
}

// startPodTerminal 啟動Pod終端連線
func (h *PodTerminalHandler) startPodTerminal(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession, shell string) error {
	// 建立管道
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	session.stdinReader = stdinReader
	session.stdinWriter = stdinWriter
	session.stdoutReader = stdoutReader
	session.stdoutWriter = stdoutWriter
	session.winSizeChan = make(chan *remotecommand.TerminalSize, 10)
	session.done = make(chan struct{})

	// 設定預設終端大小
	session.winSizeChan <- &remotecommand.TerminalSize{
		Width:  120,
		Height: 30,
	}

	// 啟動輸出讀取協程
	go h.readOutput(session)

	// 啟動Kubernetes exec
	go func() {
		defer func() {
			select {
			case <-session.done:
			default:
				close(session.done)
			}
			h.sendMessage(session.Conn, "disconnected", "Pod終端連線已斷開")
		}()

		req := client.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(session.PodName).
			Namespace(session.Namespace).
			SubResource("exec")

		req.VersionedParams(&v1.PodExecOptions{
			Container: session.Container,
			Command:   []string{shell},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(k8sConfig, "POST", req.URL())
		if err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("建立執行器失敗: %v", err))
			return
		}

		streamOption := remotecommand.StreamOptions{
			Stdin:             &terminalStream{session: session},
			Stdout:            session.stdoutWriter,
			Stderr:            session.stdoutWriter,
			TerminalSizeQueue: &terminalSizeQueue{session: session},
			Tty:               true,
		}

		if err := exec.StreamWithContext(session.Context, streamOption); err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("執行失敗: %v", err))
		}
	}()

	return nil
}

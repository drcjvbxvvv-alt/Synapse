package handlers

import (
	"math"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/shaia/Synapse/pkg/logger"
)

// handleInput 處理使用者輸入
func (h *PodTerminalHandler) handleInput(session *PodTerminalSession, input string) {
	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	// 更新最後活動時間（閒置超時計算用）
	session.lastActivityAt = time.Now()

	if session.stdinWriter != nil {
		_, err := session.stdinWriter.Write([]byte(input))
		if err != nil {
			h.sendMessage(session.Conn, "error", "寫入輸入失敗")
			return
		}
	}

	// 檢測回車鍵，標記待處理（命令將從輸出中提取）
	if h.auditService != nil && session.AuditSessionID > 0 {
		if strings.Contains(input, "\r") || strings.Contains(input, "\n") {
			session.pendingEnter = true
		} else if input == "\x03" {
			// Ctrl+C 清空當前行
			session.currentLine.Reset()
		}
	}
}

// handleResize 處理終端大小調整
func (h *PodTerminalHandler) handleResize(session *PodTerminalSession, cols, rows int) {
	if session.winSizeChan != nil {
		if cols < 0 || cols > math.MaxUint16 {
			cols = 80
		}
		if rows < 0 || rows > math.MaxUint16 {
			rows = 24
		}
		size := &remotecommand.TerminalSize{
			Width:  uint16(cols),
			Height: uint16(rows),
		}
		select {
		case session.winSizeChan <- size:
		case <-session.done:
		}
	}
}

// readOutput 讀取命令輸出
func (h *PodTerminalHandler) readOutput(session *PodTerminalSession) {
	buffer := make([]byte, wsBufferSize)
	for {
		n, err := session.stdoutReader.Read(buffer)
		if err != nil {
			break
		}

		if n > 0 {
			output := string(buffer[:n])
			h.sendMessage(session.Conn, "data", output)

			// 追蹤終端輸出，用於提取完整命令（包括Tab補全結果）
			if h.auditService != nil && session.AuditSessionID > 0 {
				h.trackOutputForCommand(session, output)
			}
		}
	}
}

// trackOutputForCommand 追蹤輸出以提取命令
func (h *PodTerminalHandler) trackOutputForCommand(session *PodTerminalSession, output string) {
	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	for _, c := range output {
		switch c {
		case '\n':
			// 遇到換行，儲存當前行並檢查是否需要記錄命令
			currentContent := session.currentLine.String()
			session.currentLine.Reset()

			if session.pendingEnter && currentContent != "" {
				// 使用者按了回車，提取命令
				cmd := h.extractCommandFromLine(currentContent)
				if cmd != "" {
					h.auditService.RecordCommandAsync(session.AuditSessionID, cmd, cmd, nil)
				}
				session.pendingEnter = false
			}
			session.lastCompleteLine = currentContent

		case '\r':
			// 回車符，可能是行首返回，暫時忽略
			continue

		case '\x1b':
			// ESC 字元，可能是 ANSI 轉義序列的開始，忽略
			continue

		case '\x07':
			// Bell 字元，忽略
			continue

		default:
			// 過濾掉不可列印字元和ANSI序列中的字元
			if c >= 32 && c < 127 {
				session.currentLine.WriteRune(c)
			}
		}
	}
}

// extractCommandFromLine 從行內容中提取命令（去掉shell提示符）
func (h *PodTerminalHandler) extractCommandFromLine(line string) string {
	// 去掉 ANSI 轉義序列
	line = h.stripANSI(line)
	line = strings.TrimSpace(line)

	if line == "" {
		return ""
	}

	// 嘗試識別並去掉常見的 shell 提示符
	// 格式如: "bash-4.4#", "root@hostname:~#", "$ ", "# ", "[user@host ~]$ "
	promptPatterns := []string{
		"# ", // root 提示符
		"$ ", // 普通使用者提示符
		"] ", // 方括號結尾的提示符
		"> ", // 其他提示符
	}

	for _, pattern := range promptPatterns {
		if idx := strings.LastIndex(line, pattern); idx != -1 {
			cmd := strings.TrimSpace(line[idx+len(pattern):])
			if cmd != "" {
				return cmd
			}
		}
	}

	// 如果沒有找到提示符模式，檢查是否看起來像命令
	// 如果行以常見命令開頭，可能就是命令本身
	commonCommands := []string{"ls", "cd", "cat", "grep", "kubectl", "find", "pwd", "echo", "ps", "top", "vi", "vim", "nano", "apt", "yum", "dnf", "pip", "npm", "go", "python", "java", "curl", "wget", "tar", "cp", "mv", "rm", "mkdir", "chmod", "chown", "df", "du", "free", "whoami", "id", "date", "tail", "head", "less", "more", "sort", "uniq", "wc", "awk", "sed", "cut", "tr", "diff", "patch", "git", "docker", "helm", "make", "sh", "bash", "exit", "clear", "history"}

	lineLower := strings.ToLower(line)
	for _, cmd := range commonCommands {
		if strings.HasPrefix(lineLower, cmd+" ") || lineLower == cmd {
			return line
		}
	}

	return ""
}

// stripANSI 去掉ANSI轉義序列
func (h *PodTerminalHandler) stripANSI(s string) string {
	// 簡單的ANSI轉義序列過濾
	result := strings.Builder{}
	inEscape := false
	for _, c := range s {
		if c == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			// ANSI序列通常以字母結尾
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(c)
	}
	return result.String()
}

// closeSession 關閉會話
func (h *PodTerminalHandler) closeSession(session *PodTerminalSession) {
	if session.stdinWriter != nil {
		_ = session.stdinWriter.Close()
	}
	if session.stdoutReader != nil {
		_ = session.stdoutReader.Close()
	}
	if session.done != nil {
		select {
		case <-session.done:
		default:
			close(session.done)
		}
	}
}

// sendMessage 傳送WebSocket訊息
func (h *PodTerminalHandler) sendMessage(conn *websocket.Conn, msgType, data string) {
	msg := PodTerminalMessage{
		Type: msgType,
		Data: data,
	}

	if err := conn.WriteJSON(msg); err != nil {
		logger.Error("傳送WebSocket訊息失敗", "error", err)
	}
}

// terminalStream 實現io.Reader和io.Writer介面
type terminalStream struct {
	session *PodTerminalSession
}

func (t *terminalStream) Read(p []byte) (int, error) {
	return t.session.stdinReader.Read(p)
}

func (t *terminalStream) Write(p []byte) (int, error) {
	return len(p), nil // 不需要寫入
}

// terminalSizeQueue 實現remotecommand.TerminalSizeQueue介面
type terminalSizeQueue struct {
	session *PodTerminalSession
}

func (t *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.session.winSizeChan:
		return size
	case <-t.session.done:
		return nil
	}
}

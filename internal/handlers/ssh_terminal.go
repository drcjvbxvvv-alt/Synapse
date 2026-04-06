package handlers

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// SSHHandler SSH終端處理器
type SSHHandler struct {
	auditService *services.AuditService
}

// NewSSHHandler 建立SSH處理器
func NewSSHHandler(auditService *services.AuditService) *SSHHandler {
	return &SSHHandler{
		auditService: auditService,
	}
}

// SSHConfig SSH連線配置
type SSHConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	AuthType   string `json:"authType"` // "password" or "key"
	ClusterID  uint   `json:"clusterId,omitempty"`
}

// SSHMessage WebSocket訊息
type SSHMessage struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data,omitempty"`
	Cols   int         `json:"cols,omitempty"`
	Rows   int         `json:"rows,omitempty"`
	Config *SSHConfig  `json:"config,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// SSHSession SSH會話資訊
type SSHSession struct {
	auditSessionID   uint
	currentLine      strings.Builder // 當前行的輸出內容
	lastCompleteLine string          // 上一個完整行
	pendingEnter     bool            // 是否有待處理的回車鍵
}

// WebSocket升級器
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return middleware.IsOriginAllowed(origin)
	},
}

// SSHConnect 處理SSH WebSocket連線
func (h *SSHHandler) SSHConnect(c *gin.Context) {
	userID := c.GetUint("user_id") // 從JWT中獲取使用者ID

	// 升級HTTP連線為WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket升級失敗", "error", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	logger.Info("SSH WebSocket連線建立")

	var sshClient *ssh.Client
	var sshSession *ssh.Session
	var stdin io.WriteCloser
	var stdout io.Reader
	var stderr io.Reader
	var sessionInfo *SSHSession

	// 清理資源
	defer func() {
		if sshSession != nil {
			_ = sshSession.Close()
		}
		if sshClient != nil {
			_ = sshClient.Close()
		}
		// 關閉審計會話
		if sessionInfo != nil && sessionInfo.auditSessionID > 0 && h.auditService != nil {
			_ = h.auditService.CloseSession(sessionInfo.auditSessionID, "closed")
		}
	}()

	for {
		var msg SSHMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			logger.Error("讀取WebSocket訊息失敗", "error", err)
			break
		}

		switch msg.Type {
		case "connect":
			if msg.Config == nil {
				h.sendError(conn, "缺少SSH配置")
				continue
			}

			// 建立審計會話
			sessionInfo = &SSHSession{}
			if h.auditService != nil {
				auditSession, err := h.auditService.CreateSession(&services.CreateSessionRequest{
					UserID:     userID,
					ClusterID:  msg.Config.ClusterID,
					TargetType: services.TerminalTypeNode,
					Node:       fmt.Sprintf("%s:%d", msg.Config.Host, msg.Config.Port),
				})
				if err != nil {
					logger.Error("建立審計會話失敗", "error", err)
				} else {
					sessionInfo.auditSessionID = auditSession.ID
				}
			}

			// 建立SSH連線
			sshClient, sshSession, stdin, stdout, stderr, err = h.createSSHConnection(msg.Config)
			if err != nil {
				h.sendError(conn, fmt.Sprintf("SSH連線失敗: %v", err))
				if sessionInfo != nil && sessionInfo.auditSessionID > 0 && h.auditService != nil {
					_ = h.auditService.CloseSession(sessionInfo.auditSessionID, "error")
				}
				continue
			}

			// 傳送連線成功訊息
			_ = conn.WriteJSON(SSHMessage{
				Type: "connected",
			})

			// 啟動輸出讀取協程
			go h.readSSHOutput(conn, stdout, stderr, sessionInfo)

		case "input":
			if stdin != nil && msg.Data != nil {
				if input, ok := msg.Data.(string); ok {
					_, err := stdin.Write([]byte(input))
					if err != nil {
						logger.Error("寫入SSH輸入失敗", "error", err)
						h.sendError(conn, "寫入輸入失敗")
					}

					// 檢測回車鍵，標記待處理
					if sessionInfo != nil && h.auditService != nil && sessionInfo.auditSessionID > 0 {
						if strings.Contains(input, "\r") || strings.Contains(input, "\n") {
							sessionInfo.pendingEnter = true
						} else if input == "\x03" {
							// Ctrl+C 清空當前行
							sessionInfo.currentLine.Reset()
						}
					}
				}
			}

		case "resize":
			if sshSession != nil && msg.Cols > 0 && msg.Rows > 0 {
				err := sshSession.WindowChange(msg.Rows, msg.Cols)
				if err != nil {
					logger.Error("調整終端大小失敗", "error", err)
				}
			}
		}
	}

	logger.Info("SSH WebSocket連線關閉")
}

// trackOutputForCommand 追蹤輸出以提取命令
func (h *SSHHandler) trackOutputForCommand(session *SSHSession, output string) {
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
					h.auditService.RecordCommandAsync(session.auditSessionID, cmd, cmd, nil)
				}
				session.pendingEnter = false
			}
			session.lastCompleteLine = currentContent

		case '\r':
			// 回車符，忽略
			continue

		case '\x1b':
			// ESC 字元，忽略
			continue

		case '\x07':
			// Bell 字元，忽略
			continue

		default:
			// 過濾掉不可列印字元
			if c >= 32 && c < 127 {
				session.currentLine.WriteRune(c)
			}
		}
	}
}

// extractCommandFromLine 從行內容中提取命令（去掉shell提示符）
func (h *SSHHandler) extractCommandFromLine(line string) string {
	// 去掉 ANSI 轉義序列
	line = h.stripANSI(line)
	line = strings.TrimSpace(line)

	if line == "" {
		return ""
	}

	// 嘗試識別並去掉常見的 shell 提示符
	promptPatterns := []string{
		"# ",
		"$ ",
		"] ",
		"> ",
	}

	for _, pattern := range promptPatterns {
		if idx := strings.LastIndex(line, pattern); idx != -1 {
			cmd := strings.TrimSpace(line[idx+len(pattern):])
			if cmd != "" {
				return cmd
			}
		}
	}

	// 檢查是否看起來像命令
	commonCommands := []string{"ls", "cd", "cat", "grep", "kubectl", "find", "pwd", "echo", "ps", "top", "vi", "vim", "nano", "apt", "yum", "dnf", "pip", "npm", "go", "python", "java", "curl", "wget", "tar", "cp", "mv", "rm", "mkdir", "chmod", "chown", "df", "du", "free", "whoami", "id", "date", "tail", "head", "less", "more", "sort", "uniq", "wc", "awk", "sed", "cut", "tr", "diff", "patch", "git", "docker", "helm", "make", "sh", "bash", "exit", "clear", "history", "systemctl", "journalctl", "service", "ifconfig", "ip", "netstat", "ss", "ping", "traceroute", "nslookup", "dig", "hostname", "uname", "uptime", "dmesg", "lsof", "kill", "pkill", "htop", "iotop", "vmstat", "iostat", "sar"}

	lineLower := strings.ToLower(line)
	for _, cmd := range commonCommands {
		if strings.HasPrefix(lineLower, cmd+" ") || lineLower == cmd {
			return line
		}
	}

	return ""
}

// stripANSI 去掉ANSI轉義序列
func (h *SSHHandler) stripANSI(s string) string {
	result := strings.Builder{}
	inEscape := false
	for _, c := range s {
		if c == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(c)
	}
	return result.String()
}

// createSSHConnection 建立SSH連線
func (h *SSHHandler) createSSHConnection(config *SSHConfig) (*ssh.Client, *ssh.Session, io.WriteCloser, io.Reader, io.Reader, error) {
	// 建立SSH客戶端配置
	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec G106 -- 平臺管理場景，目標節點已在叢集管理範圍內
		Timeout:         30 * time.Second,
	}

	// 根據認證型別設定認證方法
	switch config.AuthType {
	case "password":
		if config.Password == "" {
			return nil, nil, nil, nil, nil, fmt.Errorf("密碼不能為空")
		}
		sshConfig.Auth = []ssh.AuthMethod{
			ssh.Password(config.Password),
		}

	case "key":
		if config.PrivateKey == "" {
			return nil, nil, nil, nil, nil, fmt.Errorf("私鑰不能為空")
		}

		// 解析私鑰
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("解析私鑰失敗: %v", err)
		}

		sshConfig.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}

	default:
		return nil, nil, nil, nil, nil, fmt.Errorf("不支援的認證型別: %s", config.AuthType)
	}

	// 連線SSH伺服器
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("連線SSH伺服器失敗: %v", err)
	}

	// 建立SSH會話
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("建立SSH會話失敗: %v", err)
	}

	// 設定終端模式
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // 啟用回顯
		ssh.TTY_OP_ISPEED: 14400, // 輸入速度
		ssh.TTY_OP_OSPEED: 14400, // 輸出速度
	}

	// 請求偽終端
	err = session.RequestPty("xterm-256color", 24, 80, modes)
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("請求偽終端失敗: %v", err)
	}

	// 獲取輸入輸出流
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("獲取stdin失敗: %v", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("獲取stdout失敗: %v", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("獲取stderr失敗: %v", err)
	}

	// 啟動shell
	err = session.Shell()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("啟動shell失敗: %v", err)
	}

	return client, session, stdin, stdout, stderr, nil
}

// readSSHOutput 讀取SSH輸出
func (h *SSHHandler) readSSHOutput(conn *websocket.Conn, stdout, stderr io.Reader, session *SSHSession) {
	// 讀取stdout
	go func() {
		buffer := make([]byte, wsBufferSize)
		for {
			n, err := stdout.Read(buffer)
			if err != nil {
				if err != io.EOF {
					logger.Error("讀取SSH stdout失敗", "error", err)
				}
				break
			}

			if n > 0 {
				output := string(buffer[:n])
				err = conn.WriteJSON(SSHMessage{
					Type: "data",
					Data: output,
				})
				if err != nil {
					logger.Error("傳送SSH輸出失敗", "error", err)
					break
				}

				// 追蹤輸出以提取命令
				if session != nil && h.auditService != nil && session.auditSessionID > 0 {
					h.trackOutputForCommand(session, output)
				}
			}
		}
	}()

	// 讀取stderr
	go func() {
		buffer := make([]byte, wsBufferSize)
		for {
			n, err := stderr.Read(buffer)
			if err != nil {
				if err != io.EOF {
					logger.Error("讀取SSH stderr失敗", "error", err)
				}
				break
			}

			if n > 0 {
				err = conn.WriteJSON(SSHMessage{
					Type: "data",
					Data: string(buffer[:n]),
				})
				if err != nil {
					logger.Error("傳送SSH錯誤輸出失敗", "error", err)
					break
				}
			}
		}
	}()
}

// sendError 傳送錯誤訊息
func (h *SSHHandler) sendError(conn *websocket.Conn, errorMsg string) {
	_ = conn.WriteJSON(SSHMessage{
		Type:  "error",
		Error: errorMsg,
	})
}

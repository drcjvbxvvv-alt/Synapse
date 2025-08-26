package handlers

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"k8s-management-backend/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// SSHHandler SSH终端处理器
type SSHHandler struct{}

// NewSSHHandler 创建SSH处理器
func NewSSHHandler() *SSHHandler {
	return &SSHHandler{}
}

// SSHConfig SSH连接配置
type SSHConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	AuthType   string `json:"authType"` // "password" or "key"
}

// SSHMessage WebSocket消息
type SSHMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
	Cols int         `json:"cols,omitempty"`
	Rows int         `json:"rows,omitempty"`
	Config *SSHConfig `json:"config,omitempty"`
	Error string     `json:"error,omitempty"`
}

// WebSocket升级器
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

// SSHConnect 处理SSH WebSocket连接
func (h *SSHHandler) SSHConnect(c *gin.Context) {
	// 升级HTTP连接为WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket升级失败", "error", err)
		return
	}
	defer conn.Close()

	logger.Info("SSH WebSocket连接建立")

	var sshClient *ssh.Client
	var sshSession *ssh.Session
	var stdin io.WriteCloser
	var stdout io.Reader
	var stderr io.Reader

	// 清理资源
	defer func() {
		if sshSession != nil {
			sshSession.Close()
		}
		if sshClient != nil {
			sshClient.Close()
		}
	}()

	for {
		var msg SSHMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			logger.Error("读取WebSocket消息失败", "error", err)
			break
		}

		switch msg.Type {
		case "connect":
			if msg.Config == nil {
				h.sendError(conn, "缺少SSH配置")
				continue
			}

			// 创建SSH连接
			sshClient, sshSession, stdin, stdout, stderr, err = h.createSSHConnection(msg.Config)
			if err != nil {
				h.sendError(conn, fmt.Sprintf("SSH连接失败: %v", err))
				continue
			}

			// 发送连接成功消息
			conn.WriteJSON(SSHMessage{
				Type: "connected",
			})

			// 启动输出读取协程
			go h.readSSHOutput(conn, stdout, stderr)

		case "input":
			if stdin != nil && msg.Data != nil {
				if input, ok := msg.Data.(string); ok {
					_, err := stdin.Write([]byte(input))
					if err != nil {
						logger.Error("写入SSH输入失败", "error", err)
						h.sendError(conn, "写入输入失败")
					}
				}
			}

		case "resize":
			if sshSession != nil && msg.Cols > 0 && msg.Rows > 0 {
				err := sshSession.WindowChange(msg.Rows, msg.Cols)
				if err != nil {
					logger.Error("调整终端大小失败", "error", err)
				}
			}
		}
	}

	logger.Info("SSH WebSocket连接关闭")
}

// createSSHConnection 创建SSH连接
func (h *SSHHandler) createSSHConnection(config *SSHConfig) (*ssh.Client, *ssh.Session, io.WriteCloser, io.Reader, io.Reader, error) {
	// 创建SSH客户端配置
	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应该验证主机密钥
		Timeout:         30 * time.Second,
	}

	// 根据认证类型设置认证方法
	switch config.AuthType {
	case "password":
		if config.Password == "" {
			return nil, nil, nil, nil, nil, fmt.Errorf("密码不能为空")
		}
		sshConfig.Auth = []ssh.AuthMethod{
			ssh.Password(config.Password),
		}

	case "key":
		if config.PrivateKey == "" {
			return nil, nil, nil, nil, nil, fmt.Errorf("私钥不能为空")
		}
		
		// 解析私钥
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		
		sshConfig.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}

	default:
		return nil, nil, nil, nil, nil, fmt.Errorf("不支持的认证类型: %s", config.AuthType)
	}

	// 连接SSH服务器
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("连接SSH服务器失败: %v", err)
	}

	// 创建SSH会话
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("创建SSH会话失败: %v", err)
	}

	// 设置终端模式
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // 启用回显
		ssh.TTY_OP_ISPEED: 14400, // 输入速度
		ssh.TTY_OP_OSPEED: 14400, // 输出速度
	}

	// 请求伪终端
	err = session.RequestPty("xterm-256color", 24, 80, modes)
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("请求伪终端失败: %v", err)
	}

	// 获取输入输出流
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("获取stdin失败: %v", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("获取stdout失败: %v", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("获取stderr失败: %v", err)
	}

	// 启动shell
	err = session.Shell()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("启动shell失败: %v", err)
	}

	return client, session, stdin, stdout, stderr, nil
}

// readSSHOutput 读取SSH输出
func (h *SSHHandler) readSSHOutput(conn *websocket.Conn, stdout, stderr io.Reader) {
	// 读取stdout
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := stdout.Read(buffer)
			if err != nil {
				if err != io.EOF {
					logger.Error("读取SSH stdout失败", "error", err)
				}
				break
			}
			
			if n > 0 {
				err = conn.WriteJSON(SSHMessage{
					Type: "data",
					Data: string(buffer[:n]),
				})
				if err != nil {
					logger.Error("发送SSH输出失败", "error", err)
					break
				}
			}
		}
	}()

	// 读取stderr
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := stderr.Read(buffer)
			if err != nil {
				if err != io.EOF {
					logger.Error("读取SSH stderr失败", "error", err)
				}
				break
			}
			
			if n > 0 {
				err = conn.WriteJSON(SSHMessage{
					Type: "data",
					Data: string(buffer[:n]),
				})
				if err != nil {
					logger.Error("发送SSH错误输出失败", "error", err)
					break
				}
			}
		}
	}()
}

// sendError 发送错误消息
func (h *SSHHandler) sendError(conn *websocket.Conn, errorMsg string) {
	conn.WriteJSON(SSHMessage{
		Type:  "error",
		Error: errorMsg,
	})
}
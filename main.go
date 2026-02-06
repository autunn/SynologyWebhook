package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Config 结构体
type Config struct {
	CorpID         string `json:"corpid"`
	AgentID        string `json:"agentid"`
	CorpSecret     string `json:"corpsecret"`
	Token          string `json:"token"`
	EncodingAESKey string `json:"encoding_aes_key"`
	ProxyURL       string `json:"proxy_url"`
	NasURL         string `json:"nas_url"`
	PhotoURL       string `json:"photo_url"`
	Configured     bool   `json:"configured"`
}

var configPath = "data/config.json"
var accessToken string
var accessTokenExpiresAt int64

// sessionToken 用于验证 Cookie 的有效性
// 每次重启都会随机生成，意味着重启后所有登录都会失效（更安全）
var sessionToken string

func init() {
	// 生成随机 Session Token
	b := make([]byte, 16)
	rand.Read(b)
	sessionToken = hex.EncodeToString(b)
}

func main() {
	os.MkdirAll("data", 0755)
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	// 获取密码 (默认为 synology)
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "synology"
	}

	log.Println("Security: Session-based Auth enabled.")

	// ===========================
	// 1. 公开路由 (登录 & Webhook)
	// ===========================

	// 登录页面
	r.GET("/login", func(c *gin.Context) {
		// 如果已经登录，直接跳到首页
		if checkCookie(c) {
			c.Redirect(http.StatusFound, "/")
			return
		}
		c.HTML(http.StatusOK, "login.html", nil)
	})

	// 登录动作
	r.POST("/login", func(c *gin.Context) {
		password := c.PostForm("password")
		if password == adminPass {
			// 密码正确，设置 Cookie
			// MaxAge: 3600秒 (1小时过期), Path: "/", HttpOnly: true (防止JS窃取)
			c.SetCookie("auth_session", sessionToken, 3600*24, "/", "", false, true)
			c.Redirect(http.StatusFound, "/")
		} else {
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "密码错误，请重试"})
		}
	})

	// 退出登录
	r.GET("/logout", func(c *gin.Context) {
		c.SetCookie("auth_session", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
	})

	// Webhook 回调验证 (GET) - 必须公开
	r.GET("/webhook", func(c *gin.Context) {
		handleWebhookVerify(c)
	})

	// Webhook 接收消息 (POST) - 必须公开 (内部校验配置状态)
	r.POST("/webhook", func(c *gin.Context) {
		handleWebhookMsg(c)
	})

	// ===========================
	// 2. 私密路由组 (需要 Cookie)
	// ===========================
	authorized := r.Group("/")
	authorized.Use(AuthMiddleware())
	{
		authorized.GET("/", func(c *gin.Context) {
			conf := loadConfig()
			c.HTML(http.StatusOK, "index.html", gin.H{
				"config":  conf,
				"success": c.Query("success"),
			})
		})

		authorized.POST("/save", func(c *gin.Context) {
			handleSave(c)
		})
	}

	log.Println("Server :5080 Started")
	r.Run(":5080")
}

// ---------------------------------------------------------
// 中间件与辅助函数
// ---------------------------------------------------------

// AuthMiddleware 检查 Cookie 是否合法
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checkCookie(c) {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func checkCookie(c *gin.Context) bool {
	cookie, err := c.Cookie("auth_session")
	if err != nil {
		return false
	}
	return cookie == sessionToken
}

// 处理配置保存
func handleSave(c *gin.Context) {
	newConfig := Config{
		CorpID:         c.PostForm("corpid"),
		AgentID:        c.PostForm("agentid"),
		CorpSecret:     c.PostForm("corpsecret"),
		Token:          c.PostForm("token"),
		EncodingAESKey: c.PostForm("encoding_aes_key"),
		ProxyURL:       strings.TrimRight(c.PostForm("proxy_url"), "/"),
		NasURL:         strings.TrimRight(c.PostForm("nas_url"), "/"),
		PhotoURL:       c.PostForm("photo_url"),
		Configured:     true,
	}
	if newConfig.NasURL == "" {
		newConfig.NasURL = "http://quickconnect.to/"
	}
	saveConfig(newConfig)
	c.Redirect(http.StatusSeeOther, "/?success=true")
}

// Webhook 验证逻辑
func handleWebhookVerify(c *gin.Context) {
	conf := loadConfig()
	msgSignature := c.Query("msg_signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	echostr := c.Query("echostr")

	if msgSignature == "" {
		c.String(http.StatusBadRequest, "Invalid Request")
		return
	}

	if !verifySignature(conf.Token, timestamp, nonce, echostr, msgSignature) {
		log.Println("Sign Verify Failed")
		c.String(http.StatusForbidden, "Sign Error")
		return
	}

	decryptedMsg, err := decryptEchoStr(conf.EncodingAESKey, echostr)
	if err != nil {
		log.Printf("Decrypt Failed: %v", err)
		c.String(http.StatusForbidden, "Decrypt Error")
		return
	}

	c.String(http.StatusOK, string(decryptedMsg))
}

// Webhook 消息处理逻辑
func handleWebhookMsg(c *gin.Context) {
	var synologyData map[string]interface{}
	if err := c.ShouldBindJSON(&synologyData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON error"})
		return
	}
	conf := loadConfig()
	if !conf.Configured {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not configured"})
		return
	}
	go sendToWeChat(conf, synologyData)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ---------------------------------------------------------
// 剩下的底层函数 (签名、加密、配置加载、发微信) 保持不变
// 请确保这里包含之前的 verifySignature, decryptEchoStr,
// loadConfig, saveConfig, getAccessToken, sendToWeChat 函数
// (为了篇幅，这里假设你已经保留了它们，如果需要我再次完整贴出请告知)
// ---------------------------------------------------------

// ... 这里必须保留原来的 verifySignature 等函数 ...
// 为了代码完整性，请把之前的辅助函数都贴在 main.go 的下方
// 下面是几个必要的占位符，你的代码里要有这些的具体实现：

func verifySignature(token, timestamp, nonce, echostr, msgSignature string) bool {
	params := []string{token, timestamp, nonce, echostr}
	sort.Strings(params)
	str := strings.Join(params, "")
	h := sha1.New()
	h.Write([]byte(str))
	return fmt.Sprintf("%x", h.Sum(nil)) == msgSignature
}

func decryptEchoStr(encodingAESKey, echostr string) ([]byte, error) {
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, err
	}
	cipherText, err := base64.StdEncoding.DecodeString(echostr)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	if len(cipherText) < aes.BlockSize {
		return nil, errors.New("cipher too short")
	}
	iv := aesKey[:16]
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("cipher not block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherText, cipherText)
	pad := int(cipherText[len(cipherText)-1])
	if pad < 1 || pad > 32 {
		pad = 0
	}
	cipherText = cipherText[:len(cipherText)-pad]

	msgLen := binary.BigEndian.Uint32(cipherText[16:20])
	return cipherText[20 : 20+int(msgLen)], nil
}

func loadConfig() Config {
	var conf Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{Configured: false}
	}
	json.Unmarshal(data, &conf)
	return conf
}

func saveConfig(conf Config) {
	data, _ := json.MarshalIndent(conf, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func getAccessToken(conf Config) (string, error) {
	if accessToken != "" && accessTokenExpiresAt > time.Now().Unix()+60 {
		return accessToken, nil
	}
	baseURL := "https://qyapi.weixin.qq.com"
	if conf.ProxyURL != "" {
		baseURL = conf.ProxyURL
	}
	url := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s", baseURL, conf.CorpID, conf.CorpSecret)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if errcode, ok := result["errcode"].(float64); ok && errcode != 0 {
		return "", fmt.Errorf("API Error: %v", result["errmsg"])
	}
	if token, ok := result["access_token"].(string); ok {
		expiresIn := int64(7200)
		if exp, ok := result["expires_in"].(float64); ok {
			expiresIn = int64(exp)
		}
		accessToken = token
		accessTokenExpiresAt = time.Now().Unix() + expiresIn
		return accessToken, nil
	}
	return "", fmt.Errorf("Token Error")
}

func sendToWeChat(conf Config, data map[string]interface{}) {
	token, err := getAccessToken(conf)
	if err != nil {
		log.Println("Token Error:", err)
		return
	}
	content := "系统事件"
	if msg, ok := data["message"].(string); ok {
		content = msg
	} else if val, ok := data["data"].(map[string]interface{}); ok {
		if text, ok := val["text"].(string); ok {
			content = text
		}
	} else if text, ok := data["text"].(string); ok {
		content = text
	}

	baseURL := "https://qyapi.weixin.qq.com"
	if conf.ProxyURL != "" {
		baseURL = conf.ProxyURL
	}

	picURL := conf.PhotoURL
	if picURL == "" {
		picURL = fmt.Sprintf("https://picsum.photos/600/300?random=%d", time.Now().UnixNano())
	}
	jumpURL := conf.NasURL
	if jumpURL == "" {
		jumpURL = "https://www.synology.com"
	}
	agentID, _ := strconv.Atoi(conf.AgentID)

	payload := map[string]interface{}{
		"touser":  "@all",
		"msgtype": "news",
		"agentid": agentID,
		"news": map[string]interface{}{
			"articles": []map[string]interface{}{
				{
					"title":       "NAS 通知中心",
					"description": fmt.Sprintf("[%s]\n%s", time.Now().Format("15:04"), content),
					"url":         jumpURL,
					"picurl":      picURL,
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	postURL := fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s", baseURL, token)

	resp, err := http.Post(postURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Println("Push Error:", err)
		return
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	log.Println("WeChat Response:", string(respBody))
}

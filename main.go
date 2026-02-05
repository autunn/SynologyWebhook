package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
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

func main() {
	os.MkdirAll("data", 0755)
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	// 1. 首页
	r.GET("/", func(c *gin.Context) {
		conf := loadConfig()
		c.HTML(http.StatusOK, "index.html", gin.H{
			"config":  conf,
			"success": c.Query("success"),
		})
	})

	// 2. 保存配置
	r.POST("/save", func(c *gin.Context) {
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
	})

	// 3. Webhook 回调验证 (GET)
	r.GET("/webhook", func(c *gin.Context) {
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
	})

	// 4. Webhook 接收消息 (POST)
	r.POST("/webhook", func(c *gin.Context) {
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
	})

	log.Println("Server :5080 Started")
	r.Run(":5080")
}

// 签名校验
func verifySignature(token, timestamp, nonce, echostr, msgSignature string) bool {
	params := []string{token, timestamp, nonce, echostr}
	sort.Strings(params)
	str := strings.Join(params, "")
	h := sha1.New()
	h.Write([]byte(str))
	return fmt.Sprintf("%x", h.Sum(nil)) == msgSignature
}

// 解密
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
	
	// 【关键修复点】
	// 1. 获取内容长度 (位于 16-20 字节)
	msgLen := binary.BigEndian.Uint32(cipherText[16:20])
	
	// 2. 截取真正的内容 (从第 20 字节开始)
	// 必须强制转换 int(msgLen)，否则编译报错
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
	http.Post(fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s", baseURL, token), "application/json", bytes.NewBuffer(body))
}
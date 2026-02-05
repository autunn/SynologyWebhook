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

// Config 增加 Token 和 EncodingAESKey
type Config struct {
	CorpID         string `json:"corpid"`
	AgentID        string `json:"agentid"`
	CorpSecret     string `json:"corpsecret"`
	Token          string `json:"token"`            // 新增：回调Token
	EncodingAESKey string `json:"encoding_aes_key"` // 新增：回调解密Key
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
			Token:          c.PostForm("token"),            // 保存 Token
			EncodingAESKey: c.PostForm("encoding_aes_key"), // 保存 AES Key
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

	// 3. Webhook 回调验证接口 (GET) - 严格遵循企业微信文档
	r.GET("/webhook", func(c *gin.Context) {
		conf := loadConfig()
		
		// 获取 URL 参数
		msgSignature := c.Query("msg_signature")
		timestamp := c.Query("timestamp")
		nonce := c.Query("nonce")
		echostr := c.Query("echostr")

		// 如果没有这些参数，说明不是回调验证，忽略
		if msgSignature == "" || timestamp == "" || nonce == "" || echostr == "" {
			c.String(http.StatusBadRequest, "Invalid Request")
			return
		}

		// 1. 校验签名
		if !verifySignature(conf.Token, timestamp, nonce, echostr, msgSignature) {
			log.Println("签名校验失败")
			c.String(http.StatusForbidden, "Signature verification failed")
			return
		}

		// 2. 解密 echostr
		decryptedMsg, err := decryptEchoStr(conf.EncodingAESKey, echostr)
		if err != nil {
			log.Printf("解密失败: %v\n", err)
			c.String(http.StatusForbidden, "Decryption failed")
			return
		}

		// 3. 返回解密后的明文
		c.String(http.StatusOK, string(decryptedMsg))
	})

	// 4. Webhook 接收消息接口 (POST)
	r.POST("/webhook", func(c *gin.Context) {
		var synologyData map[string]interface{}
		if err := c.ShouldBindJSON(&synologyData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "JSON invalid"})
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

	log.Println("Go Server Started on :5080")
	r.Run(":5080")
}

// --- 以下是企业微信加解密核心逻辑 ---

// 校验签名
func verifySignature(token, timestamp, nonce, echostr, msgSignature string) bool {
	// 排序
	params := []string{token, timestamp, nonce, echostr}
	sort.Strings(params)
	
	// 拼接
	str := strings.Join(params, "")
	
	// SHA1 哈希
	h := sha1.New()
	h.Write([]byte(str))
	calculatedSignature := fmt.Sprintf("%x", h.Sum(nil))
	
	return calculatedSignature == msgSignature
}

// 解密 echostr
func decryptEchoStr(encodingAESKey, echostr string) ([]byte, error) {
	// 1. Base64 解码 AESKey
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, err
	}

	// 2. Base64 解码密文
	cipherText, err := base64.StdEncoding.DecodeString(echostr)
	if err != nil {
		return nil, err
	}

	// 3. AES 解密
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	if len(cipherText) < aes.BlockSize {
		return nil, errors.New("cipher text too short")
	}

	// IV 是 Key 的前 16 位
	iv := aesKey[:16]
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("cipher text is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherText, cipherText)

	// 4. 去除填充 (PKCS7)
	pad := int(cipherText[len(cipherText)-1])
	if pad < 1 || pad > 32 {
		pad = 0
	}
	cipherText = cipherText[:len(cipherText)-pad]

	// 5. 去除 16 位随机字符串
	content := cipherText[16:]

	// 6. 读取 4 位长度
	msgLen := binary.BigEndian.Uint32(content[:4])
	
	// 7. 截取真正的明文
	return content[4 : 4+msgLen], nil
}

// --- 基础逻辑维持不变 ---

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

func getBaseURL(conf Config) string {
	if conf.ProxyURL != "" {
		return conf.ProxyURL
	}
	return "https://qyapi.weixin.qq.com"
}

func getAccessToken(conf Config) (string, error) {
	if accessToken != "" && accessTokenExpiresAt > time.Now().Unix()+60 {
		return accessToken, nil
	}
	baseURL := getBaseURL(conf)
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
		accessToken = token
		accessTokenExpiresAt = time.Now().Unix() + 7000
		return accessToken, nil
	}
	return "", fmt.Errorf("Token Error")
}

func sendToWeChat(conf Config, data map[string]interface{}) {
	token, err := getAccessToken(conf)
	if err != nil {
		log.Println("Token Fail:", err)
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

	agentID, _ := strconv.Atoi(conf.AgentID)
	baseURL := getBaseURL(conf)

	picURL := conf.PhotoURL
	if picURL == "" {
		picURL = fmt.Sprintf("https://picsum.photos/600/300?random=%d", time.Now().UnixNano())
	}

	jumpURL := conf.NasURL
	if jumpURL == "" {
		jumpURL = "https://www.synology.com"
	}

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
		log.Println("Push Fail:", err)
		return
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	log.Println("WeChat Resp:", string(respBody))
}
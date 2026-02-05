package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Config 结构体：增加了代理、标题、接收人
type Config struct {
	CorpID      string `json:"corpid"`
	AgentID     string `json:"agentid"`
	CorpSecret  string `json:"corpsecret"`
	ProxyURL    string `json:"proxy_url"` // 新增：代理地址
	CustomTitle string `json:"title"`     // 新增：消息标题
	ToUser      string `json:"touser"`    // 新增：接收用户
	Configured  bool   `json:"configured"`
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

	// 2. 保存配置（增加了新字段的处理）
	r.POST("/save", func(c *gin.Context) {
		newConfig := Config{
			CorpID:      c.PostForm("corpid"),
			AgentID:     c.PostForm("agentid"),
			CorpSecret:  c.PostForm("corpsecret"),
			ProxyURL:    strings.TrimRight(c.PostForm("proxy_url"), "/"), // 去掉末尾斜杠
			CustomTitle: c.PostForm("title"),
			ToUser:      c.PostForm("touser"),
			Configured:  true,
		}
		// 设置默认值
		if newConfig.ToUser == "" {
			newConfig.ToUser = "@all"
		}
		saveConfig(newConfig)
		c.Redirect(http.StatusSeeOther, "/?success=true")
	})

	// 3. Webhook 接口
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
		c.JSON(http.StatusOK, gin.H{"status": "processing"})
	})

	log.Println("Full-Feature Go Server starting on :5080")
	r.Run(":5080")
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

// 辅助函数：获取基础 URL（支持代理）
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

	// 动态拼接 URL
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
		var expiresIn int64 = 7200
		if exp, ok := result["expires_in"].(float64); ok {
			expiresIn = int64(exp)
		}
		accessToken = token
		accessTokenExpiresAt = time.Now().Unix() + expiresIn
		return accessToken, nil
	}
	return "", fmt.Errorf("Token not found")
}

func sendToWeChat(conf Config, data map[string]interface{}) {
	token, err := getAccessToken(conf)
	if err != nil {
		log.Println("Token Error:", err)
		return
	}

	// 智能解析内容
	content := "收到新通知"
	if msg, ok := data["message"].(string); ok {
		content = msg
	} else if val, ok := data["data"].(map[string]interface{}); ok {
		if text, ok := val["text"].(string); ok {
			content = text
		}
	} else if text, ok := data["text"].(string); ok {
		content = text
	}

	// 标题处理
	title := "NAS 系统通知"
	if conf.CustomTitle != "" {
		title = conf.CustomTitle
	}

	agentID, _ := strconv.Atoi(conf.AgentID)
	baseURL := getBaseURL(conf)

	// 构造消息
	payload := map[string]interface{}{
		"touser":  conf.ToUser,
		"msgtype": "textcard",
		"agentid": agentID,
		"textcard": map[string]interface{}{
			"title":       title,
			"description": fmt.Sprintf("<div class=\"gray\">%s</div> <div class=\"normal\">%s</div>", time.Now().Format("15:04:05"), content),
			"url":         "https://www.synology.com",
			"btntxt":      "详情",
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
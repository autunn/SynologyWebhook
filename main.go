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
	"time"

	"github.com/gin-gonic/gin"
)

// Config 结构体用于保存配置
type Config struct {
	CorpID     string `json:"corpid"`
	AgentID    string `json:"agentid"`
	CorpSecret string `json:"corpsecret"`
	Configured bool   `json:"configured"`
}

var configPath = "data/config.json"
var accessToken string
var accessTokenExpiresAt int64

func main() {
	// 确保数据目录存在
	os.MkdirAll("data", 0755)

	// 设置为发布模式
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 加载 HTML 模板
	r.LoadHTMLGlob("templates/*")

	// 1. 首页配置界面
	r.GET("/", func(c *gin.Context) {
		conf := loadConfig()
		c.HTML(http.StatusOK, "index.html", gin.H{
			"config":  conf,
			"success": c.Query("success"),
		})
	})

	// 2. 保存配置接口
	r.POST("/save", func(c *gin.Context) {
		newConfig := Config{
			CorpID:     c.PostForm("corpid"),
			AgentID:    c.PostForm("agentid"),
			CorpSecret: c.PostForm("corpsecret"),
			Configured: true,
		}
		saveConfig(newConfig)
		c.Redirect(http.StatusSeeOther, "/?success=true")
	})

	// 3. Webhook 接收接口
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

	log.Println("Server starting on :5080")
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

func getAccessToken(corpID, corpSecret string) (string, error) {
	if accessToken != "" && accessTokenExpiresAt > time.Now().Unix()+60 {
		return accessToken, nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", corpID, corpSecret)
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
		// 安全获取 expires_in
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
	token, err := getAccessToken(conf.CorpID, conf.CorpSecret)
	if err != nil {
		log.Println("Token Error:", err)
		return
	}

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

	agentID, _ := strconv.Atoi(conf.AgentID)

	payload := map[string]interface{}{
		"touser":  "@all",
		"msgtype": "textcard",
		"agentid": agentID,
		"textcard": map[string]interface{}{
			"title":       "NAS 通知",
			"description": fmt.Sprintf("<div class=\"gray\">%s</div> <div class=\"normal\">%s</div>", time.Now().Format("15:04:05"), content),
			"url":         "https://www.synology.com",
			"btntxt":      "详情",
		},
	}

	body, _ := json.Marshal(payload)
	postURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	
	resp, err := http.Post(postURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Println("Push Error:", err)
		return
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	log.Println("WeChat Response:", string(respBody))
}
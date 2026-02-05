package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	os.MkdirAll("data", 0755)

	// 设置为发布模式，减少日志输出
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

	// 2. 保存配置
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

	// 3. 接收群晖 Webhook 并转发
	r.POST("/webhook", func(c *gin.Context) {
		var synologyData map[string]interface{}
		if err := c.ShouldBindJSON(&synologyData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "JSON格式错误"})
			return
		}

		conf := loadConfig()
		if !conf.Configured {
			c.JSON(http.StatusForbidden, gin.H{"error": "未配置企业微信参数"})
			return
		}

		// 异步推送，提高响应速度
		go sendToWeChat(conf, synologyData)
		c.JSON(http.StatusOK, gin.H{"status": "received"})
	})

	log.Println("SynologyWebhook Go version started on :5080")
	r.Run(":5080")
}

func loadConfig() Config {
	var conf Config
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Config{Configured: false}
	}
	json.Unmarshal(data, &conf)
	return conf
}

func saveConfig(conf Config) {
	data, _ := json.MarshalIndent(conf, "", "  ")
	ioutil.WriteFile(configPath, data, 0644)
}

func getAccessToken(corpID, corpSecret string) (string, error) {
	if accessToken != "" && accessTokenExpiresAt > time.Now().Unix()+60 {
		return accessToken, nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", corpID, corpSecret)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return "", fmt.Errorf("请求令牌失败")
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if token, ok := result["access_token"].(string); ok {
		accessToken = token
		accessTokenExpiresAt = time.Now().Unix() + 7000
		return accessToken, nil
	}
	return "", fmt.Errorf("解析令牌失败")
}

func sendToWeChat(conf Config, data map[string]interface{}) {
	token, err := getAccessToken(conf.CorpID, conf.CorpSecret)
	if err != nil {
		log.Println("Push Error:", err)
		return
	}

	// 提取群晖通知内容
	content := "收到来自群晖的通知"
	if msg, ok := data["text"].(string); ok {
		content = msg
	} else if val, ok := data["data"].(map[string]interface{}); ok {
		if text, ok := val["text"].(string); ok {
			content = text
		}
	}

	agentID, _ := strconv.Atoi(conf.AgentID)
	// 组装卡片消息，加入随机风景图
	payload := map[string]interface{}{
		"touser":  "@all",
		"msgtype": "textcard",
		"agentid": agentID,
		"textcard": map[string]interface{}{
			"title":       "NAS 系统通知",
			"description": fmt.Sprintf("<div class=\"gray\">%s</div><div class=\"normal\">%s</div>", time.Now().Format("2006-01-02 15:04:05"), content),
			"url":         "https://www.synology.com",
			"btntxt":      "更多详情",
		},
	}

	body, _ := json.Marshal(payload)
	http.Post(fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token), "application/json", bytes.NewBuffer(body))
}
package main

import (
	"bufio"
	"bytes"
	"c-chat/middleware"
	"c-chat/model"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.Use(middleware.CORSMiddleware()) //跨域中间件
	// 设置路由
	r.POST("/api/v1/chat", func(c *gin.Context) {
		// 解析客户端请求
		var clientReq model.ClientRequest
		if err := c.ShouldBindJSON(&clientReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 构造DeepSeek请求
		deepSeekReq := model.DeepSeekRequest{
			Model:       "deepseek-chat",
			Messages:    clientReq.Messages,
			Temperature: clientReq.Temperature,
			MaxTokens:   clientReq.MaxTokens,
			Stream:      true,
		}

		// 调用DeepSeek API
		response, err := callDeepSeekAPI(deepSeekReq)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		// 处理API错误
		//if response.Error.Message != "" {
		//	c.JSON(http.StatusBadGateway, gin.H{"error": response.Error.Message, "status": "error"})
		//	return
		//}

		// 返回成功响应
		c.JSON(http.StatusOK, gin.H{
			"status":   "success",
			"response": response,
		})
	})

	r.Run(":9998")
}

func callDeepSeekAPI(req model.DeepSeekRequest) (*[]byte, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("missing DeepSeek API key")
	}

	requestBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 60 * time.Second}

	httpReq, err := http.NewRequest(
		"POST",
		"https://api.deepseek.com/v1/chat/completions",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Println("请求失败:", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		fmt.Println("请求失败，状态码:", resp.Status)
		return nil, nil
	}

	// 逐行解析 SSE 响应
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fmt.Println("收到数据:", line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("读取流失败:", err)
		os.Exit(1)
	}

	// 打印响应体
	//body, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	return nil, err
	//}
	//fmt.Println("Response Body:", string(body))

	return nil, nil
}

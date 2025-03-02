package utils

import (
	"bufio"
	"bytes"
	"c-chat/model"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"time"
)

/*
WebSocket工具类
*/
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 允许跨域连接（生产环境请配置更严格的规则）
}

// 处理 WebSocket 连接
func HandleWebSocket(c *gin.Context) error {

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return errors.New("missing DeepSeek API key")
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("WebSocket 连接升级失败:", err)
		return err
	}
	defer conn.Close()

	fmt.Println("WebSocket 连接成功！")
	// 监听客户端消息
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("WebSocket 读取消息失败:", err)
			break
		}
		fmt.Println("收到消息:", string(msg))

		// 这里可以解析 JSON 并根据 type 处理 （向客户端返回数据）
		//conn.WriteMessage(websocket.TextMessage, []byte("收到："+string(msg)))
		var clientReq model.DeepSeekRequest
		if err := json.Unmarshal(msg, &clientReq); err != nil {
			fmt.Println("消息反序列化失败:", err)
			return err
		}
		// 构造DeepSeek请求
		deepSeekReq := model.DeepSeekRequest{
			Model:       "deepseek-reasoner",
			Messages:    clientReq.Messages,
			Temperature: clientReq.Temperature,
			MaxTokens:   clientReq.MaxTokens,
			Stream:      true,
		}

		requestBody, err := json.Marshal(deepSeekReq)
		if err != nil {
			return err
		}

		httpReq, err := http.NewRequest(
			"POST",
			"https://api.deepseek.com/v1/chat/completions",
			bytes.NewBuffer(requestBody),
		)
		if err != nil {
			return err
		}

		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		client := &http.Client{Timeout: 60 * time.Second}

		resp, err := client.Do(httpReq)
		if err != nil {
			fmt.Println("请求 DeepSeek 失败:", err)
			return err
		}
		defer resp.Body.Close()
		// 检查响应状态
		if resp.StatusCode != http.StatusOK {
			fmt.Println("请求失败，状态码:", resp.Status)
			return errors.New("请求失败，")
		}

		// 逐行解析 SSE 响应
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) < 6 || line[:5] != "data:" {
				continue
			}
			//data: {"id":"63b8f39d-3ac2-4d3e-845b-6a5d9b39fcf5",
			//"object":"chat.completion.chunk",
			//"created":1740887645,
			//"model":"deepseek-chat",
			//"system_fingerprint":"fp_3a5770e1b4_prod0225",
			//"choices":[{"index":0,"delta":{"role":"assistant","content":""},"logprobs":null,"finish_reason":null}]}
			// 解析 SSE 数据
			var sse model.SSEData
			jsonData := line[6:]
			if err := json.Unmarshal([]byte(jsonData), &sse); err == nil {
				for _, choice := range sse.Choices {
					message := choice.Delta.Content
					if message != "" {
						// 发送到 WebSocket
						conn.WriteMessage(websocket.TextMessage, []byte(message))
					}
				}
			}
		}

		fmt.Println("流式数据推送完成，WebSocket 连接关闭。")
		return nil
	}
	return nil
}

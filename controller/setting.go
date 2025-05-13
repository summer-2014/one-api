package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/middleware"
	"github.com/songquanpeng/one-api/model"
)

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func UpdateSensitiveFilterSetting(c *gin.Context) {
	var setting Setting
	err := json.NewDecoder(c.Request.Body).Decode(&setting)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	
	if setting.Key == "SensitiveFilterEnabled" {
		err := model.UpdateOption("SensitiveFilterEnabled", setting.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	} else if setting.Key == "SensitiveWords" {
		// 更新敏感词列表
		middleware.UpdateSensitiveWords(setting.Value)
		// 更新数据库中的配置（可选，因为已经保存到文件中）
		err := model.UpdateOption("SensitiveWords", setting.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	} else if setting.Key == "SensitiveFilterResponse" {
		// 更新敏感词响应
		middleware.UpdateSensitiveResponse(setting.Value)
		// 更新数据库中的配置（可选，因为已经保存到文件中）
		err := model.UpdateOption("SensitiveFilterResponse", setting.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	} else {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的设置项",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
} 
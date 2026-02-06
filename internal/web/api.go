package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/undeadpelmen/new-client/internal/terrarium"
)

type WebAPI struct {
	terrarium  *terrarium.Terrarium
	controller *terrarium.TerrariumController
}

func NewWebAPI(terrarium *terrarium.Terrarium, controller *terrarium.TerrariumController) *WebAPI {
	return &WebAPI{
		terrarium:  terrarium,
		controller: controller,
	}
}

func (api *WebAPI) getState(c *gin.Context) {
	state := api.terrarium.GetState()

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"timestamp": time.Now().Format(time.RFC3339),
			"sensors": gin.H{
				"temperature":  state.CurrentTemp,
				"humidity":     state.CurrentHumidity,
				"last_read":    state.LastSensorRead.Format(time.RFC3339),
				"sensor_error": state.SensorError,
			},
			"relays": gin.H{
				"light":         state.LightRelay,
				"heater":        state.HeaterRelay,
				"pump":          state.PumpRelay,
				"last_pump_run": state.LastPumpRun.Format(time.RFC3339),
			},
			"system": gin.H{
				"cycle_count": state.CycleCount,
				"uptime":      int(time.Since(state.Uptime).Seconds()),
				"mode":        state.SystemMode,
			},
		},
	})
}

func (api *WebAPI) getHistory(c *gin.Context) {
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	history := api.terrarium.GetHistory(limit)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   history,
		"meta": gin.H{
			"count": len(history),
			"total": api.terrarium.GetHistoryCount(),
			"limit": limit,
		},
	})
}

func (api *WebAPI) getSettings(c *gin.Context) {
	settings := api.terrarium.GetSettings()
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   settings,
	})
}

func (api *WebAPI) updateSettings(c *gin.Context) {
	var updateData map[string]interface{}

	if err := c.BindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Неверный формат данных",
		})
		return
	}

	api.terrarium.UpdateSettings(func(s *terrarium.TerrariumSettings) {
		if lightSchedule, ok := updateData["light_schedule"].(map[string]interface{}); ok {
			if start, ok := lightSchedule["start_time"].(string); ok {
				s.LightSchedule.StartTime = start
			}
			if end, ok := lightSchedule["end_time"].(string); ok {
				s.LightSchedule.EndTime = end
			}
			if enabled, ok := lightSchedule["enabled"].(bool); ok {
				s.LightSchedule.Enabled = enabled
			}
		}

		if targets, ok := updateData["targets"].(map[string]interface{}); ok {
			if temp, ok := targets["temperature"].(float64); ok {
				s.Targets.Temperature = float32(temp)
			}
			if humidity, ok := targets["humidity"].(float64); ok {
				s.Targets.Humidity = float32(humidity)
			}
		}

		if pump, ok := updateData["pump_settings"].(map[string]interface{}); ok {
			if duration, ok := pump["duration_seconds"].(float64); ok {
				s.PumpSettings.DurationSeconds = int(duration)
			}
			if interval, ok := pump["min_interval"].(float64); ok {
				s.PumpSettings.MinInterval = int(interval)
			}
		}

		if pause, ok := updateData["cycle_pause"].(float64); ok {
			s.CyclePause = int(pause)
		}

		if mock, ok := updateData["use_mock_data"].(bool); ok {
			s.UseMockData = mock
		}
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Настройки обновлены",
	})
}

func (api *WebAPI) resetSettings(c *gin.Context) {
	api.terrarium.ResetSettings()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Настройки сброшены к значениям по умолчанию",
	})
}

func (api *WebAPI) getHealth(c *gin.Context) {
	state := api.terrarium.GetState()

	healthStatus := "healthy"
	if state.SystemMode == "critical" {
		healthStatus = "critical"
	} else if state.SystemMode == "error" {
		healthStatus = "degraded"
	} else if state.SensorError {
		healthStatus = "warning"
	}

	c.JSON(http.StatusOK, gin.H{
		"status": healthStatus,
		"components": gin.H{
			"dht22_sensor": func() string {
				if state.SensorError {
					return "error"
				}
				return "ok"
			}(),
			"relays":       "ok",
			"control_loop": "running",
			"api_server":   "running",
		},
		"uptime_seconds": int(time.Since(state.Uptime).Seconds()),
		"cycle_count":    state.CycleCount,
		"version":        "2.0.0",
		"timestamp":      time.Now().Format(time.RFC3339),
	})
}

func (api *WebAPI) toggleMockData(c *gin.Context) {
	settings := api.terrarium.GetSettings()
	api.terrarium.UpdateSettings(func(s *terrarium.TerrariumSettings) {
		s.UseMockData = !settings.UseMockData
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Режим имитации: %v", !settings.UseMockData),
	})
}

func (api *WebAPI) testSensor(c *gin.Context) {
	reading, err := api.controller.TestSensor()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Ошибка датчика: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"temperature": reading.Temperature,
			"humidity":    reading.Humidity,
			"valid":       reading.Valid,
			"timestamp":   time.Now().Format(time.RFC3339),
		},
	})
}

func (api *WebAPI) SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	})

	apiRoute := router.Group("/api/v1")
	{
		apiRoute.GET("/state", api.getState)
		apiRoute.GET("/history", api.getHistory)
		apiRoute.GET("/settings", api.getSettings)
		apiRoute.PUT("/settings", api.updateSettings)
		apiRoute.POST("/settings/reset", api.resetSettings)
		apiRoute.GET("/health", api.getHealth)
		apiRoute.POST("/mock", api.toggleMockData)
		apiRoute.GET("/sensor/test", api.testSensor)
	}

	router.StaticFile("/", "./web/static/index.html")
	router.Static("/static", "./web/static")

	return router
}

package terrarium

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/undeadpelmen/new-client/internal/display"
	"github.com/undeadpelmen/new-client/internal/gpio"
	"github.com/undeadpelmen/new-client/internal/sensor"
)

type TerrariumController struct {
	terrarium    *Terrarium
	relays       *gpio.RelayController
	sensor       *sensor.DHT22
	display      *display.OLEDDisplay
	mockTemp     float32
	mockHumidity float32
	mockTempDir  float32
	mockMu       sync.RWMutex
}

func NewTerrariumController(terrarium *Terrarium, relays *gpio.RelayController) *TerrariumController {
	var dht22 *sensor.DHT22
	if relays != nil {
		dht22 = sensor.NewDHT22(relays.GetDHT22Pin(), "GPIO4")
	}

	// Initialize OLED display
	var oledDisplay *display.OLEDDisplay
	if relays != nil {
		oled, err := display.NewOLEDDisplay()
		if err != nil {
			log.Printf("Failed to initialize OLED display: %v", err)
			oledDisplay = nil
		} else {
			if err := oled.Init(); err != nil {
				log.Printf("Failed to initialize OLED display: %v", err)
				oledDisplay = nil
			} else if err := oled.Close(); err != nil {
				log.Printf("Failed to close OLED display: %v", err)
				oledDisplay = nil
			} else {
				oledDisplay = oled
				log.Println("OLED display initialized successfully")
			}
		}
	}

	return &TerrariumController{
		terrarium:    terrarium,
		relays:       relays,
		sensor:       dht22,
		display:      oledDisplay,
		mockTemp:     25.0,
		mockHumidity: 65.0,
		mockTempDir:  0.1,
	}
}

func (tc *TerrariumController) readMockSensorData() (temp, humidity float32, err error) {
	tc.mockMu.Lock()
	defer tc.mockMu.Unlock()

	tc.mockTemp += tc.mockTempDir
	if tc.mockTemp > 28.0 || tc.mockTemp < 22.0 {
		tc.mockTempDir = -tc.mockTempDir
	}
	tc.mockHumidity += 0.2
	if tc.mockHumidity > 80.0 {
		tc.mockHumidity = 50.0
	}
	log.Printf("Mock data: T=%.1f°C, H=%.1f%%", tc.mockTemp, tc.mockHumidity)
	return tc.mockTemp, tc.mockHumidity, nil
}

func (tc *TerrariumController) readRealSensorData() (temp, humidity float32, err error) {
	if tc.sensor == nil {
		return 0, 0, fmt.Errorf("DHT22 sensor not initialized")
	}
	reading, err := tc.sensor.ReadWithRetry(2)
	if err != nil {
		log.Printf("DHT22 read error: %v", err)
		return 0, 0, err
	}
	log.Printf("DHT22 data: T=%.1f°C, H=%.1f%%",
		reading.Temperature, reading.Humidity)
	return reading.Temperature, reading.Humidity, nil
}

func (tc *TerrariumController) ReadSensorData() (temp, humidity float32, err error) {
	useMock := tc.terrarium.GetSettings().UseMockData

	if useMock {
		return tc.readMockSensorData()
	}
	return tc.readRealSensorData()
}

func (tc *TerrariumController) ShouldLightBeOn() bool {
	settings := tc.terrarium.GetSettings()

	if !settings.LightSchedule.Enabled {
		return false
	}

	now := time.Now()
	hour, minute, _ := now.Clock()
	currentMinutes := hour*60 + minute

	var startHour, startMin int
	n, err := fmt.Sscanf(settings.LightSchedule.StartTime, "%d:%d", &startHour, &startMin)
	if err != nil || n != 2 {
		log.Printf("Failed to parse start time '%s': %v", settings.LightSchedule.StartTime, err)
		return false
	}
	if startHour < 0 || startHour > 23 || startMin < 0 || startMin > 59 {
		log.Printf("Invalid start time values: hour=%d, min=%d", startHour, startMin)
		return false
	}
	startMinutes := startHour*60 + startMin

	var endHour, endMin int
	n, err = fmt.Sscanf(settings.LightSchedule.EndTime, "%d:%d", &endHour, &endMin)
	if err != nil || n != 2 {
		log.Printf("Failed to parse end time '%s': %v", settings.LightSchedule.EndTime, err)
		return false
	}
	if endHour < 0 || endHour > 23 || endMin < 0 || endMin > 59 {
		log.Printf("Invalid end time values: hour=%d, min=%d", endHour, endMin)
		return false
	}
	endMinutes := endHour*60 + endMin

	if startMinutes > endMinutes {
		return currentMinutes >= startMinutes || currentMinutes <= endMinutes
	}

	return currentMinutes >= startMinutes && currentMinutes <= endMinutes
}

func (tc *TerrariumController) ControlLoop(ctx context.Context) {
	log.Println("Starting terrarium control loop")

	tc.terrarium.UpdateState(func(s *TerrariumState) {
		s.Uptime = time.Now()
		s.SystemMode = "auto"
	})

	errorCount := 0
	const maxErrors = 5

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping control loop")
			if tc.relays != nil {
				if err := tc.relays.SetLight(false); err != nil {
					log.Printf("Error turning off light during shutdown: %v", err)
				}
				if err := tc.relays.SetHeater(false); err != nil {
					log.Printf("Error turning off heater during shutdown: %v", err)
				}
				if err := tc.relays.SetPump(false); err != nil {
					log.Printf("Error turning off pump during shutdown: %v", err)
				}
			}
			return

		default:
			lightShouldBeOn := tc.ShouldLightBeOn()

			var currentLightState bool
			tc.terrarium.UpdateState(func(s *TerrariumState) {
				currentLightState = s.LightRelay
			})

			if lightShouldBeOn != currentLightState {
				if tc.relays != nil {
					if err := tc.relays.SetLight(lightShouldBeOn); err != nil {
						log.Printf("Light control error: %v", err)
						tc.terrarium.UpdateState(func(s *TerrariumState) {
							s.SystemMode = "error"
						})
						errorCount++
					} else {
						log.Printf("Lighting: %v", lightShouldBeOn)
					}
				}

				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.LightRelay = lightShouldBeOn
				})
			}

			temp, humidity, err := tc.ReadSensorData()
			if err != nil {
				log.Printf("Sensor read error: %v", err)
				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.SensorError = true
					s.SystemMode = "error"
				})
				errorCount++

				tc.terrarium.UpdateState(func(s *TerrariumState) {
					temp = s.CurrentTemp
					humidity = s.CurrentHumidity
				})
			} else {
				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.SensorError = false
					s.SystemMode = "auto"
				})
				errorCount = 0
			}

			settings := tc.terrarium.GetSettings()
			targetTemp := settings.Targets.Temperature

			heaterShouldBeOn := temp < targetTemp

			var currentHeaterState bool
			tc.terrarium.UpdateState(func(s *TerrariumState) {
				currentHeaterState = s.HeaterRelay
			})

			if heaterShouldBeOn != currentHeaterState {
				if tc.relays != nil {
					if err := tc.relays.SetHeater(heaterShouldBeOn); err != nil {
						log.Printf("Heater control error: %v", err)
					} else if heaterShouldBeOn {
						log.Printf("Heater turned on (T=%.1f < %.1f)", temp, targetTemp)
					} else {
						log.Printf("Heater turned off")
					}
				}

				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.HeaterRelay = heaterShouldBeOn
				})
			}

			targetHumidity := settings.Targets.Humidity
			//pumpDuration := settings.PumpSettings.DurationSeconds
			//pumpMinInterval := settings.PumpSettings.MinInterval

			//var lastPumpRun time.Time
			var currentPumpState bool
			tc.terrarium.UpdateState(func(s *TerrariumState) {
				//lastPumpRun = s.LastPumpRun
				currentPumpState = s.PumpRelay
			})

			pumpShouldBeOn := humidity < targetHumidity

			if pumpShouldBeOn && !currentPumpState {
				if tc.relays != nil {
					if err := tc.relays.SetPump(true); err != nil {
						log.Printf("Pump turn-on error: %v", err)
					} else {
						log.Printf("Pump turned on (H=%.1f < %.1f)",
							humidity, targetHumidity)

						tc.terrarium.UpdateState(func(s *TerrariumState) {
							s.LastPumpRun = time.Now()
						})
					}
				}

				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.PumpRelay = true
				})
			} else if !pumpShouldBeOn && currentPumpState {
				if tc.relays != nil {
					if err := tc.relays.SetPump(false); err != nil {
						log.Printf("Pump turn-off error: %v", err)
					} else {
						log.Printf("Pump turned off (H=%.1f > %.1f)", humidity, targetHumidity)
					}
				}

				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.PumpRelay = false
				})
			}

			tc.terrarium.UpdateState(func(s *TerrariumState) {
				s.CurrentTemp = temp
				s.CurrentHumidity = humidity
				s.LastSensorRead = time.Now()
				s.CycleCount++

				if errorCount >= maxErrors {
					s.SystemMode = "critical"
					log.Printf("Critical mode! %d consecutive errors", errorCount)
				}
			})

			// Update OLED display with sensor data
			if tc.display != nil {
				if err := tc.display.DisplaySensorData(temp, humidity); err != nil {
					log.Printf("Display update error: %v", err)
				}
			}

			tc.terrarium.AddHistoryRecord(HistoricalRecord{
				Timestamp:   time.Now(),
				Temperature: temp,
				Humidity:    humidity,
				LightOn:     lightShouldBeOn,
				HeaterOn:    heaterShouldBeOn,
				PumpOn:      pumpShouldBeOn,
				SensorError: tc.terrarium.GetState().SensorError,
			})

			pause := settings.CyclePause
			if errorCount > 0 {
				pause = pause * 2
				log.Printf("Increased pause %d sec due to errors", pause)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(pause) * time.Second):
			}
		}
	}
}

func (tc *TerrariumController) TestSensor() (*sensor.DHT22Reading, error) {
	if tc.sensor == nil {
		return nil, fmt.Errorf("DHT22 sensor not initialized")
	}
	return tc.sensor.ReadWithRetry(3)
}

func (tc *TerrariumController) Close() error {
	if tc.display != nil {
		if err := tc.display.Close(); err != nil {
			log.Printf("Error closing display: %v", err)
			return err
		}
		log.Println("OLED display closed")
	}
	return nil
}

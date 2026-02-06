package terrarium

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/undeadpelmen/new-client/internal/gpio"
	"github.com/undeadpelmen/new-client/internal/sensor"
)

type TerrariumController struct {
	terrarium    *Terrarium
	relays       *gpio.RelayController
	sensor       *sensor.DHT22
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

	return &TerrariumController{
		terrarium:    terrarium,
		relays:       relays,
		sensor:       dht22,
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
	log.Printf("Мок-данные: T=%.1f°C, H=%.1f%%", tc.mockTemp, tc.mockHumidity)
	return tc.mockTemp, tc.mockHumidity, nil
}

func (tc *TerrariumController) readRealSensorData() (temp, humidity float32, err error) {
	if tc.sensor == nil {
		return 0, 0, fmt.Errorf("датчик DHT22 не инициализирован")
	}
	reading, err := tc.sensor.ReadWithRetry(2)
	if err != nil {
		log.Printf("Ошибка чтения DHT22: %v", err)
		return 0, 0, err
	}
	log.Printf("Данные DHT22: T=%.1f°C, H=%.1f%%",
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
	hour, min, _ := now.Clock()
	currentMinutes := hour*60 + min

	var startHour, startMin int
	fmt.Sscanf(settings.LightSchedule.StartTime, "%d:%d", &startHour, &startMin)
	startMinutes := startHour*60 + startMin

	var endHour, endMin int
	fmt.Sscanf(settings.LightSchedule.EndTime, "%d:%d", &endHour, &endMin)
	endMinutes := endHour*60 + endMin

	if startMinutes > endMinutes {
		return currentMinutes >= startMinutes || currentMinutes <= endMinutes
	}

	return currentMinutes >= startMinutes && currentMinutes <= endMinutes
}

func (tc *TerrariumController) ControlLoop(ctx context.Context) {
	log.Println("Запуск цикла управления террариумом")

	tc.terrarium.UpdateState(func(s *TerrariumState) {
		s.Uptime = time.Now()
		s.SystemMode = "auto"
	})

	errorCount := 0
	const maxErrors = 5

	for {
		select {
		case <-ctx.Done():
			log.Println("Остановка цикла управления")
			if tc.relays != nil {
				tc.relays.SetLight(false)
				tc.relays.SetHeater(false)
				tc.relays.SetPump(false)
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
						log.Printf("Ошибка управления светом: %v", err)
						tc.terrarium.UpdateState(func(s *TerrariumState) {
							s.SystemMode = "error"
						})
						errorCount++
					} else {
						log.Printf("Освещение: %v", lightShouldBeOn)
					}
				}

				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.LightRelay = lightShouldBeOn
				})
			}

			temp, humidity, err := tc.ReadSensorData()
			if err != nil {
				log.Printf("Ошибка чтения датчика: %v", err)
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
						log.Printf("Ошибка управления нагревателем: %v", err)
					} else if heaterShouldBeOn {
						log.Printf("Нагреватель включен (T=%.1f < %.1f)", temp, targetTemp)
					}
				}

				tc.terrarium.UpdateState(func(s *TerrariumState) {
					s.HeaterRelay = heaterShouldBeOn
				})
			}

			targetHumidity := settings.Targets.Humidity
			pumpDuration := settings.PumpSettings.DurationSeconds
			pumpMinInterval := settings.PumpSettings.MinInterval

			var lastPumpRun time.Time
			var currentPumpState bool
			tc.terrarium.UpdateState(func(s *TerrariumState) {
				lastPumpRun = s.LastPumpRun
				currentPumpState = s.PumpRelay
			})

			pumpShouldBeOn := false
			if humidity < targetHumidity {
				if time.Since(lastPumpRun) > time.Duration(pumpMinInterval)*time.Second {
					pumpShouldBeOn = true
				}
			}

			if pumpShouldBeOn && !currentPumpState {
				if tc.relays != nil {
					if err := tc.relays.SetPump(true); err != nil {
						log.Printf("Ошибка включения помпы: %v", err)
					} else {
						log.Printf("Помпа включена на %d сек (H=%.1f < %.1f)",
							pumpDuration, humidity, targetHumidity)

						go func(duration int) {
							select {
							case <-ctx.Done():
								return
							case <-time.After(time.Duration(duration) * time.Second):
								if tc.relays != nil {
									if err := tc.relays.SetPump(false); err != nil {
										log.Printf("Ошибка выключения помпы: %v", err)
									} else {
										log.Println("Помпа выключена")
									}
								}
							}
						}(pumpDuration)

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
						log.Printf("Ошибка выключения помпы: %v", err)
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
					log.Printf("Критический режим! %d ошибок подряд", errorCount)
				}
			})

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
				log.Printf("Увеличенная пауза %d сек из-за ошибок", pause)
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
		return nil, fmt.Errorf("датчик DHT22 не инициализирован")
	}
	return tc.sensor.ReadWithRetry(3)
}

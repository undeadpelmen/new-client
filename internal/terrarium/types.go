package terrarium

import (
	"sync"
	"time"
)

type TerrariumState struct {
	mu              sync.RWMutex
	CurrentTemp     float32   `json:"temperature"`
	CurrentHumidity float32   `json:"humidity"`
	LightRelay      bool      `json:"light_on"`
	HeaterRelay     bool      `json:"heater_on"`
	PumpRelay       bool      `json:"pump_on"`
	LastSensorRead  time.Time `json:"last_read"`
	LastPumpRun     time.Time `json:"last_pump_run"`
	CycleCount      int64     `json:"cycle_count"`
	Uptime          time.Time `json:"uptime"`
	SystemMode      string    `json:"system_mode"`
	SensorError     bool      `json:"sensor_error"`
}

type TerrariumSettings struct {
	mu            sync.RWMutex
	LightSchedule struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Enabled   bool   `json:"enabled"`
	} `json:"light_schedule"`
	Targets struct {
		Temperature float32 `json:"temperature"`
		Humidity    float32 `json:"humidity"`
	} `json:"targets"`
	PumpSettings struct {
		DurationSeconds int `json:"duration_seconds"`
		MinInterval     int `json:"min_interval"`
	} `json:"pump_settings"`
	CyclePause  int  `json:"cycle_pause"`
	UseMockData bool `json:"use_mock_data"`
}

type HistoricalRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	Temperature float32   `json:"temperature"`
	Humidity    float32   `json:"humidity"`
	LightOn     bool      `json:"light_on"`
	HeaterOn    bool      `json:"heater_on"`
	PumpOn      bool      `json:"pump_on"`
	SensorError bool      `json:"sensor_error"`
}

type Terrarium struct {
	state     *TerrariumState
	settings  *TerrariumSettings
	history   []HistoricalRecord
	historyMu sync.RWMutex
}

func NewTerrarium() *Terrarium {
	state := &TerrariumState{
		Uptime:     time.Now(),
		SystemMode: "auto",
	}

	settings := &TerrariumSettings{}
	settings.LightSchedule.StartTime = "08:00"
	settings.LightSchedule.EndTime = "20:00"
	settings.LightSchedule.Enabled = true
	settings.Targets.Temperature = 26.0
	settings.Targets.Humidity = 70.0
	settings.PumpSettings.DurationSeconds = 10
	settings.PumpSettings.MinInterval = 300
	settings.CyclePause = 30
	settings.UseMockData = false

	return &Terrarium{
		state:    state,
		settings: settings,
		history:  make([]HistoricalRecord, 0),
	}
}

func (t *Terrarium) GetState() *TerrariumState {
	t.state.mu.RLock()
	defer t.state.mu.RUnlock()
	return t.state
}

func (t *Terrarium) UpdateState(updater func(*TerrariumState)) {
	t.state.mu.Lock()
	updater(t.state)
	t.state.mu.Unlock()
}

func (t *Terrarium) GetSettings() *TerrariumSettings {
	t.settings.mu.RLock()
	defer t.settings.mu.RUnlock()
	return t.settings
}

func (t *Terrarium) UpdateSettings(updater func(*TerrariumSettings)) {
	t.settings.mu.Lock()
	updater(t.settings)
	t.settings.mu.Unlock()
}

func (t *Terrarium) AddHistoryRecord(record HistoricalRecord) {
	t.historyMu.Lock()
	t.history = append(t.history, record)
	if len(t.history) > 1000 {
		t.history = t.history[len(t.history)-1000:]
	}
	t.historyMu.Unlock()
}

func (t *Terrarium) GetHistory(limit int) []HistoricalRecord {
	t.historyMu.RLock()
	defer t.historyMu.RUnlock()

	if limit <= 0 || limit > len(t.history) {
		limit = len(t.history)
	}

	recentHistory := t.history
	if len(t.history) > limit {
		recentHistory = t.history[len(t.history)-limit:]
	}

	result := make([]HistoricalRecord, len(recentHistory))
	copy(result, recentHistory)
	return result
}

func (t *Terrarium) GetHistoryCount() int {
	t.historyMu.RLock()
	defer t.historyMu.RUnlock()
	return len(t.history)
}

func (t *Terrarium) ResetSettings() {
	t.UpdateSettings(func(s *TerrariumSettings) {
		s.LightSchedule.StartTime = "08:00"
		s.LightSchedule.EndTime = "20:00"
		s.LightSchedule.Enabled = true
		s.Targets.Temperature = 26.0
		s.Targets.Humidity = 70.0
		s.PumpSettings.DurationSeconds = 10
		s.PumpSettings.MinInterval = 300
		s.CyclePause = 30
		s.UseMockData = false
	})
}

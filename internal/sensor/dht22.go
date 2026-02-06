package sensor

import (
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
)

type DHT22Reading struct {
	Temperature float32
	Humidity    float32
	Valid       bool
}

type DHT22 struct {
	pin    gpio.PinIO
	pinNum string
}

func NewDHT22(pin gpio.PinIO, pinNum string) *DHT22 {
	return &DHT22{pin: pin, pinNum: pinNum}
}

func delayMicroseconds(us int) {
	if us <= 0 {
		return
	}
	if us < 1000 {
		start := time.Now()
		for time.Since(start) < time.Duration(us)*time.Microsecond {
		}
	} else {
		time.Sleep(time.Duration(us) * time.Microsecond)
	}
}

func waitForPinState(pin gpio.PinIO, state gpio.Level, timeoutUs int) bool {
	start := time.Now()
	for pin.Read() != state {
		if time.Since(start) >= time.Duration(timeoutUs)*time.Microsecond {
			return false
		}
	}
	return true
}

func (d *DHT22) sendStartSignal() error {
	if err := d.pin.Out(gpio.Low); err != nil {
		return err
	}
	delayMicroseconds(18000)
	d.pin.Out(gpio.High)
	delayMicroseconds(40)
	return d.pin.In(gpio.PullUp, gpio.NoEdge)
}

func (d *DHT22) readBit() (byte, error) {
	if !waitForPinState(d.pin, gpio.Low, 100) {
		return 0, fmt.Errorf("timeout waiting for bit start")
	}
	if !waitForPinState(d.pin, gpio.High, 100) {
		return 0, fmt.Errorf("timeout waiting for high")
	}
	start := time.Now()
	for d.pin.Read() == gpio.High {
		if time.Since(start) > 100*time.Microsecond {
			break
		}
	}
	duration := time.Since(start)
	if duration > 50*time.Microsecond {
		return 1, nil
	}
	return 0, nil
}

func (d *DHT22) read40Bits() ([]byte, error) {
	data := make([]byte, 5)
	if !waitForPinState(d.pin, gpio.Low, 100) {
		return nil, fmt.Errorf("sensor response timeout (low)")
	}
	if !waitForPinState(d.pin, gpio.High, 100) {
		return nil, fmt.Errorf("sensor response timeout (high)")
	}
	for i := 0; i < 40; i++ {
		bit, err := d.readBit()
		if err != nil {
			return nil, fmt.Errorf("error reading bit %d: %v", i, err)
		}
		byteIndex := i / 8
		bitPosition := 7 - (i % 8)
		data[byteIndex] |= bit << bitPosition
	}
	return data, nil
}

func (d *DHT22) verifyChecksum(data []byte) bool {
	if len(data) != 5 {
		return false
	}
	sum := uint16(data[0]) + uint16(data[1]) + uint16(data[2]) + uint16(data[3])
	return (sum & 0xFF) == uint16(data[4])
}

func (d *DHT22) parseData(data []byte) (*DHT22Reading, error) {
	if len(data) != 5 {
		return nil, fmt.Errorf("invalid data length: %d", len(data))
	}
	humidityInt := int16(data[0])
	humidityFrac := int16(data[1])
	humidity := float32(humidityInt) + float32(humidityFrac)/10.0
	tempInt := int16(data[2] & 0x7F)
	tempFrac := int16(data[3])
	temperature := float32(tempInt) + float32(tempFrac)/10.0
	if data[2]&0x80 != 0 {
		temperature = -temperature
	}
	if humidity < 0 || humidity > 100 {
		return nil, fmt.Errorf("humidity out of range: %.1f", humidity)
	}
	if temperature < -40 || temperature > 80 {
		return nil, fmt.Errorf("temperature out of range: %.1f", temperature)
	}
	return &DHT22Reading{
		Temperature: temperature,
		Humidity:    humidity,
		Valid:       true,
	}, nil
}

func (d *DHT22) Read() (*DHT22Reading, error) {
	if err := d.sendStartSignal(); err != nil {
		return nil, fmt.Errorf("start signal failed: %v", err)
	}
	data, err := d.read40Bits()
	if err != nil {
		return nil, fmt.Errorf("read bits failed: %v", err)
	}
	if !d.verifyChecksum(data) {
		return nil, fmt.Errorf("checksum mismatch")
	}
	return d.parseData(data)
}

func (d *DHT22) ReadWithRetry(maxAttempts int) (*DHT22Reading, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		reading, err := d.Read()
		if err == nil && reading.Valid {
			return reading, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			time.Sleep(2 * time.Second)
		}
	}
	return nil, fmt.Errorf("failed after %d attempts: %v", maxAttempts, lastErr)
}

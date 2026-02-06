package gpio

import (
	"fmt"
	"log"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/rpi"
)

type RelayController struct {
	pinLight  gpio.PinOut
	pinHeater gpio.PinOut
	pinPump   gpio.PinOut
	pinDHT22  gpio.PinIO
}

func NewRelayController() (*RelayController, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("ошибка инициализации periph: %v", err)
	}

	pinLight := rpi.P1_11
	pinHeater := rpi.P1_13
	pinPump := rpi.P1_15
	pinDHT22 := rpi.P1_7

	if err := pinLight.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("ошибка настройки пина света: %v", err)
	}
	if err := pinHeater.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("ошибка настройки пина нагревателя: %v", err)
	}
	if err := pinPump.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("ошибка настройки пина помпы: %v", err)
	}

	log.Println("GPIO инициализированы успешно")
	log.Printf("Пины: Свет=%v, Нагреватель=%v, Помпа=%v, DHT22=%v",
		pinLight, pinHeater, pinPump, pinDHT22)

	return &RelayController{
		pinLight:  pinLight,
		pinHeater: pinHeater,
		pinPump:   pinPump,
		pinDHT22:  pinDHT22,
	}, nil
}

func (rc *RelayController) SetLight(state bool) error {
	if state {
		if err := rc.pinLight.Out(gpio.High); err != nil {
			return err
		}
	} else {
		if err := rc.pinLight.Out(gpio.Low); err != nil {
			return err
		}
	}
	return nil
}

func (rc *RelayController) SetHeater(state bool) error {
	if state {
		if err := rc.pinHeater.Out(gpio.High); err != nil {
			return err
		}
	} else {
		if err := rc.pinHeater.Out(gpio.Low); err != nil {
			return err
		}
	}
	return nil
}

func (rc *RelayController) SetPump(state bool) error {
	if state {
		if err := rc.pinPump.Out(gpio.High); err != nil {
			return err
		}
	} else {
		if err := rc.pinPump.Out(gpio.Low); err != nil {
			return err
		}
	}
	return nil
}

func (rc *RelayController) GetDHT22Pin() gpio.PinIO {
	return rc.pinDHT22
}

func (rc *RelayController) Shutdown() {
	log.Println("Выключаю все реле...")
	rc.SetLight(false)
	rc.SetHeater(false)
	rc.SetPump(false)
	time.Sleep(100 * time.Millisecond)
}

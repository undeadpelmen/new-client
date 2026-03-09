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
		return nil, fmt.Errorf("periph initialization error: %v", err)
	}

	pinLight := rpi.P1_11
	pinHeater := rpi.P1_13
	pinPump := rpi.P1_15
	pinDHT22 := rpi.P1_7

	if err := pinLight.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("light pin setup error: %v", err)
	}
	if err := pinHeater.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("heater pin setup error: %v", err)
	}
	if err := pinPump.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("pump pin setup error: %v", err)
	}

	log.Println("GPIO initialized successfully")
	log.Printf("Pins: Light=%v, Heater=%v, Pump=%v, DHT22=%v",
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
	log.Println("Turning off all relays...")
	rc.SetLight(false)
	rc.SetHeater(false)
	rc.SetPump(false)
	time.Sleep(100 * time.Millisecond)
}

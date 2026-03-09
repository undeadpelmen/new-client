package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/undeadpelmen/new-client/internal/gpio"
	"github.com/undeadpelmen/new-client/internal/terrarium"
	"github.com/undeadpelmen/new-client/internal/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Terrarium control system v2.0")

	terrariumInstance := terrarium.NewTerrarium()

	var relayController *gpio.RelayController
	var err error

	relayController, err = gpio.NewRelayController()
	if err != nil {
		log.Printf("GPIO initialization error: %v", err)
		log.Println("Switching to simulation mode")
		terrariumInstance.UpdateSettings(func(s *terrarium.TerrariumSettings) {
			s.UseMockData = true
		})
		relayController = nil
	} else {
		log.Println("GPIO initialized successfully")
	}

	controller := terrarium.NewTerrariumController(terrariumInstance, relayController)

	if relayController != nil {
		log.Println("Testing DHT22 sensor...")
		if reading, err := controller.TestSensor(); err != nil {
			log.Printf("DHT22 not responding: %v", err)
			log.Println("Switching to simulation mode")
			terrariumInstance.UpdateSettings(func(s *terrarium.TerrariumSettings) {
				s.UseMockData = true
			})
		} else {
			log.Printf("DHT22 working: T=%.1f°C, H=%.1f%%",
				reading.Temperature, reading.Humidity)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Starting main control loop...")
	go controller.ControlLoop(ctx)

	webAPI := web.NewWebAPI(terrariumInstance, controller)
	router := webAPI.SetupRouter()

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Println("HTTP server starting on port 8080")
		log.Println("Web interface: http://localhost:8080")
		log.Println("API: http://localhost:8080/api/v1/state")

		if terrariumInstance.GetSettings().UseMockData {
			log.Println("Mode: SIMULATION")
		} else {
			log.Println("Mode: REAL SENSOR")
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("System started. Ctrl+C to stop")

	<-sigChan
	log.Println("Shutdown signal received")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	if relayController != nil {
		relayController.Shutdown()
	}

	log.Println("System stopped gracefully")
}

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
	log.Println("Система управления террариумом v2.0")

	terrariumInstance := terrarium.NewTerrarium()

	var relayController *gpio.RelayController
	var err error

	relayController, err = gpio.NewRelayController()
	if err != nil {
		log.Printf("Ошибка инициализации GPIO: %v", err)
		log.Println("Переключаюсь в режим имитации")
		terrariumInstance.UpdateSettings(func(s *terrarium.TerrariumSettings) {
			s.UseMockData = true
		})
		relayController = nil
	} else {
		log.Println("GPIO успешно инициализированы")
	}

	controller := terrarium.NewTerrariumController(terrariumInstance, relayController)

	if relayController != nil {
		log.Println("Тестирование датчика DHT22...")
		if reading, err := controller.TestSensor(); err != nil {
			log.Printf("DHT22 не отвечает: %v", err)
			log.Println("Переключаюсь в режим имитации")
			terrariumInstance.UpdateSettings(func(s *terrarium.TerrariumSettings) {
				s.UseMockData = true
			})
		} else {
			log.Printf("DHT22 работает: T=%.1f°C, H=%.1f%%",
				reading.Temperature, reading.Humidity)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Запуск основного цикла управления...")
	go controller.ControlLoop(ctx)

	webAPI := web.NewWebAPI(terrariumInstance, controller)
	router := webAPI.SetupRouter()

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Println("HTTP сервер запускается на порту 8080")
		log.Println("Веб-интерфейс: http://localhost:8080")
		log.Println("API: http://localhost:8080/api/v1/state")

		if terrariumInstance.GetSettings().UseMockData {
			log.Println("Режим: ИМИТАЦИЯ")
		} else {
			log.Println("Режим: РЕАЛЬНЫЙ ДАТЧИК")
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка HTTP сервера: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Система запущена. Ctrl+C для остановки")

	<-sigChan
	log.Println("Получен сигнал завершения")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Ошибка при остановке HTTP сервера: %v", err)
	}

	if relayController != nil {
		relayController.Shutdown()
	}

	log.Println("Система остановлена корректно")
}

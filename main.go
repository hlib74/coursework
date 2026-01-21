package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// DevicePayload представляє JSON-навантаження для POST-запиту
type DevicePayload struct {
	DeviceName  string `json:"DeviceName"`
	DeviceType  string `json:"DeviceType"`
	IPAddress   string `json:"IPAddress"`
	RoutingType string `json:"RoutingType"`
}

var (
	logFileName = "server.log"
	fileMutex   sync.Mutex
)

func main() {
	// Запуска сервер у горутині
	go startServer()

	time.Sleep(1 * time.Second)

	// Симуляція клієнта
	runSimulation()

	fmt.Println("\nСервер продовжує працювати. Натисніть Enter щоб вийти...")
	fmt.Scanln()
}

// --- РЕАЛІЗАЦІЯ СЕРВЕРА ---

func startServer() {
	http.HandleFunc("/", handleRoot)
	fmt.Println("Сервер слухає на порту 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Сервер не вдалося запустити: %v", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleGet(w, r)
	case http.MethodPost:
		handlePost(w, r)
	case http.MethodDelete:
		handleDelete(w, r)
	default:
		http.Error(w, "Метод не дозволено", http.StatusMethodNotAllowed)
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	content, err := os.ReadFile(logFileName)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Файл логів порожній або не існує"))
		return
	}
	if err != nil {
		http.Error(w, "Не вдалося прочитати файл логів", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	var payload DevicePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Невірний формат JSON", http.StatusBadRequest)
		return
	}

	timestamp := time.Now().Format(time.RFC3339)
	logEntry := fmt.Sprintf("[%s] Name=%s, Type=%s, IP=%s, Routing=%s\n",
		timestamp, payload.DeviceName, payload.DeviceType, payload.IPAddress, payload.RoutingType)

	fileMutex.Lock()
	defer fileMutex.Unlock()

	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Не вдалося відкрити файл логів", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(logEntry); err != nil {
		http.Error(w, "Не вдалося записати у файл логів", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Дані успішно записано"))
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	if err := os.Truncate(logFileName, 0); err != nil {
		// Якщо файл не існує, це технічно успіх для "очищення"
		if !os.IsNotExist(err) {
			http.Error(w, "Не вдалося очистити файл логів", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Файл логів очищено"))
}

// --- СИМУЛЯЦІЯ КЛІЄНТА ---

type NetworkConfig struct {
	SubnetPrefix string
	PC           int
	Laptop       int
	Printer      int
}

func runSimulation() {
	networks := []NetworkConfig{
		{SubnetPrefix: "192.168.1", PC: 3, Laptop: 1, Printer: 1},
		{SubnetPrefix: "192.168.2", PC: 3, Laptop: 1, Printer: 1},
	}

	client := &http.Client{Timeout: 5 * time.Second}
	serverURL := "http://localhost:8080/"

	fmt.Println("Запуск симуляції мережі...")

	for netIdx, netArg := range networks {
		devices := []struct {
			Type  string
			Count int
		}{
			{"PC", netArg.PC},
			{"Laptop", netArg.Laptop},
			{"Printer", netArg.Printer},
		}

		ipCounter := 10

		for _, devGroup := range devices {
			for i := 1; i <= devGroup.Count; i++ {
				deviceName := fmt.Sprintf("%s%d_%d", devGroup.Type, i, netIdx+1)
				ipAddress := fmt.Sprintf("%s.%d", netArg.SubnetPrefix, ipCounter)
				routingType := "Static"
				if (ipCounter % 2) == 0 {
					routingType = "Dynamic"
				}

				payload := DevicePayload{
					DeviceName:  deviceName,
					DeviceType:  devGroup.Type,
					IPAddress:   ipAddress,
					RoutingType: routingType,
				}

				sendPostRequest(client, serverURL, payload)

				ipCounter++
			}
		}
	}

	fmt.Println("Симуляцію завершено.")
}

func sendPostRequest(client *http.Client, url string, data DevicePayload) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Помилка маршалізації JSON для %s: %v", data.DeviceName, err)
		return
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Помилка надсилання POST для %s: %v", data.DeviceName, err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("[УСПІХ] Надіслано %s (%s): Відповідь сервера: %s\n", data.DeviceName, data.IPAddress, string(body))
	} else {
		fmt.Printf("[НЕВДАЧА] Надіслано %s (%s): Сервер повернув %d\n", data.DeviceName, data.IPAddress, resp.StatusCode)
	}
}

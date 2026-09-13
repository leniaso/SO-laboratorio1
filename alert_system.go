package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"system-monitor/monitor"
)

// EstadoAlertas guarda información que debe persistir entre ejecuciones del programa
type EstadoAlertas struct {
	UltimoEnvio     map[string]time.Time `json:"ultimo_envio"`
	ContadorCPUAlta int                  `json:"contador_cpu_alta"`
}

const rutaEstado = "system_monitor_logs/alert_state.json"
const rutaAlertsLog = "system_monitor_logs/alerts.log"
const ventanaFlood = 5 * time.Minute

const colorRojo = "\033[31m"
const colorAmarillo = "\033[33m"
const colorReset = "\033[0m"

func main() {
	modoDaemon := flag.Bool("daemon", false, "Ejecutar en modo daemon (revisión continua)")
	flag.Parse()

	err := os.MkdirAll("system_monitor_logs", 0755)
	if err != nil {
		fmt.Println("Error creando directorio:", err)
		return
	}

	if *modoDaemon {
		fmt.Println("Alert system en modo daemon. (Ctrl+C para detener)")
		for {
			verificarAlertas()
			time.Sleep(1 * time.Minute)
		}
	} else {
		verificarAlertas()
	}
}

// cargarEstado lee el estado guardado en JSON, o devuelve uno vacío si no existe todavía
func cargarEstado() EstadoAlertas {
	estado := EstadoAlertas{
		UltimoEnvio: make(map[string]time.Time),
	}

	datos, err := os.ReadFile(rutaEstado)
	if err != nil {
		return estado 
	}

	err = json.Unmarshal(datos, &estado)
	if err != nil {
		return EstadoAlertas{UltimoEnvio: make(map[string]time.Time)}
	}

	if estado.UltimoEnvio == nil {
		estado.UltimoEnvio = make(map[string]time.Time)
	}

	return estado
}

// guardarEstado escribe el estado actualizado de vuelta al archivo JSON
func guardarEstado(estado EstadoAlertas) error {
	datos, err := json.MarshalIndent(estado, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rutaEstado, datos, 0644)
}

// registrarAlerta imprime en consola (con color) y escribe en alerts.log,
func registrarAlerta(estado *EstadoAlertas, tipo string, nivel string, mensaje string) error {
	ahora := time.Now()

	ultimaVez, existe := estado.UltimoEnvio[tipo]
	if existe && ahora.Sub(ultimaVez) < ventanaFlood {
		return nil // todavía dentro de la ventana anti-flood, no repetimos la alerta
	}

	color := colorAmarillo
	if nivel == "CRITICAL" {
		color = colorRojo
	}

	fmt.Printf("%s[%s] %s: %s%s\n", color, nivel, tipo, mensaje, colorReset)

	file, err := os.OpenFile(rutaAlertsLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	linea := fmt.Sprintf("%s [%s] %s: %s\n", ahora.Format("2006-01-02 15:04:05"), nivel, tipo, mensaje)
	_, err = file.WriteString(linea)
	if err != nil {
		return err
	}

	estado.UltimoEnvio[tipo] = ahora
	return nil
}

func verificarAlertas() {
	estado := cargarEstado()

	// Umbrales configurables vía "main_monitor --config" (system_monitor_logs/config.json).
	// Si no existe el archivo todavía, LeerConfig devuelve los valores por defecto del enunciado.
	config, err := monitor.LeerConfig()
	if err != nil {
		fmt.Println("Aviso: no se pudo leer config.json, usando umbrales por defecto:", err)
	}

	// 1. RAM > umbral configurado (90% por defecto)
	memTotal, memAvailable, err := monitor.ObtenerMemoria()
	if err == nil {
		porcentaje := (float64(memTotal-memAvailable) / float64(memTotal)) * 100
		if porcentaje > config.UmbralRAMPercent {
			registrarAlerta(&estado, "RAM", "CRITICAL",
				fmt.Sprintf("Uso de memoria en %.2f%% (umbral: %.0f%%)", porcentaje, config.UmbralRAMPercent))
		}
	} else {
		fmt.Println("Error obteniendo memoria:", err)
	}

	// 2. Load average > umbral configurado durante N comprobaciones consecutivas (5 y 3 por defecto)
	load1, _, _, err := monitor.ObtenerCargaPromedio()
	if err == nil {
		if load1 > config.UmbralLoadCPU {
			estado.ContadorCPUAlta++
		} else {
			estado.ContadorCPUAlta = 0
		}

		if estado.ContadorCPUAlta >= config.ChequesConsecutivos {
			registrarAlerta(&estado, "CPU_LOAD", "CRITICAL",
				fmt.Sprintf("Load average en %.2f durante %d comprobaciones consecutivas (umbral: %.1f)",
					load1, estado.ContadorCPUAlta, config.UmbralLoadCPU))
		}
	} else {
		fmt.Println("Error obteniendo load average:", err)
	}

	// 3. Disco > umbral configurado (85% por defecto)
	usoDisco, err := monitor.ObtenerUsoDisco("/")
	if err == nil {
		if usoDisco > config.UmbralDiscoPercent {
			registrarAlerta(&estado, "DISCO", "WARNING",
				fmt.Sprintf("Uso de disco en %.2f%% (umbral: %.0f%%)", usoDisco, config.UmbralDiscoPercent))
		}
	} else {
		fmt.Println("Error obteniendo uso de disco:", err)
	}

	// 4. Interfaz de red caída
	interfazCaida, err := monitor.InterfaceCaida()
	if err == nil {
		if interfazCaida != "" {
			registrarAlerta(&estado, "RED_INTERFAZ", "CRITICAL",
				fmt.Sprintf("Interfaz %s está caída (DOWN)", interfazCaida))
		}
	} else {
		fmt.Println("Error verificando interfaces:", err)
	}

	err = guardarEstado(estado)
	if err != nil {
		fmt.Println("Error guardando estado de alertas:", err)
	}
}
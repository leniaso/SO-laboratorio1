package main

import (
	"flag"
	"fmt"
	"os"
	"time"
	"system-monitor/monitor"
)

func main() {
	modoDaemon := flag.Bool("daemon", false, "Ejecutar en modo daemon (recolección continua cada 5 minutos)")
	modoPromedio := flag.Bool("promedio", false, "Calcular promedio de memoria de la última hora")
	modoFuga := flag.Bool("fuga", false, "Verificar posible fuga de memoria")
	modoProcesos := flag.Bool("procesos", false, "Mostrar top procesos por CPU/memoria")
	modoRed := flag.Bool("red", false, "Mostrar estado de la red: conexiones, errores y drops")
	flag.Parse()

	fmt.Println("Advanced System Monitor esta corriendo...")

	err := os.MkdirAll("system_monitor_logs", 0755)
	if err != nil {
		fmt.Println("Error creando directorio:", err)
		return
	}
	fmt.Println("Directorio para logs listo")

	switch {
	case *modoDaemon:
		ejecutarDaemon()

	case *modoPromedio:
		entradas, err := monitor.LeerLogCompleto()
		if err != nil {
			fmt.Println("Error leyendo el log:", err)
			return
		}
		promedio, err := monitor.CalcularPromedioMemoria(entradas, 1*time.Hour)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Printf("Promedio de memoria (última hora): %.2f%%\n", promedio)

	case *modoFuga:
		resultado, err := monitor.DetectarFugaMemoria(15.0)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Printf("Memoria actual: %.2f%% | Promedio última hora: %.2f%% | Diferencia: %.2f\n",
			resultado.MemActual, resultado.PromedioHora, resultado.Diferencia)
		if resultado.HayFuga {
			fmt.Println("⚠️  POSIBLE FUGA DE MEMORIA DETECTADA")
		} else {
			fmt.Println("✅ Uso de memoria dentro de lo normal")
		}

	case *modoProcesos:
		procesos, err := monitor.ObtenerTopProcesos(5)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println("Top 5 procesos por uso de CPU:")
		for _, p := range procesos {
			apariciones, _ := monitor.ContarApariciones(p.Nombre)
			fmt.Printf("  PID %s | %-20s | CPU: %.2f%% | MEM: %.2f%% | Visto %d veces antes en el top\n",
				p.PID, p.Nombre, p.CPU, p.Mem, apariciones)
		}
		err = monitor.GuardarTopProcesosCSV(procesos)
		if err != nil {
			fmt.Println("Error guardando procesos:", err)
			return
		}

	case *modoRed:
		interfaces, err := monitor.ObtenerInterfacesRed()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		conexiones, err := monitor.ObtenerConexionesActivas()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		_, _, erroresRxTotal, dropsRxTotal := monitor.CalcularTotalesRed(interfaces)

		fmt.Println("Estado de la red:")
		for _, iface := range interfaces {
			fmt.Printf("  Interfaz %-8s | RX: %d bytes | TX: %d bytes | Errores: %d | Drops: %d\n",
				iface.Nombre, iface.BytesRecibidos, iface.BytesEnviados, iface.ErroresRecibidos, iface.DropsRecibidos)
		}
		fmt.Printf("\nConexiones activas: %d total | %d establecidas | %d en escucha\n",
			conexiones.Total, conexiones.Establecidas, conexiones.Escuchando)
		fmt.Printf("Errores RX totales: %d | Drops RX totales: %d\n", erroresRxTotal, dropsRxTotal)

	default:
		err = recolectarDatos()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	}
}

// recolectarDatos hace una ronda completa de recolección: memoria, CPU, red, y guarda en CSV
func recolectarDatos() error {
	memTotal, memAvailable, err := monitor.ObtenerMemoria()
	if err != nil {
		return fmt.Errorf("error al obtener memoria: %w", err)
	}
	memUsada := memTotal - memAvailable
	porcentajeUso := (float64(memUsada) / float64(memTotal)) * 100

	cpuUso, err := monitor.ObtenerUsoCPU()
	if err != nil {
		return fmt.Errorf("error al obtener CPU: %w", err)
	}

	interfaces, err := monitor.ObtenerInterfacesRed()
	if err != nil {
		return fmt.Errorf("error al obtener info de red: %w", err)
	}

	fmt.Printf("[%s] Memoria: %.2f%% | CPU: %.2f%%\n",
		time.Now().Format("15:04:05"), porcentajeUso, cpuUso)

	err = monitor.GuardarLogCSV(memTotal, memAvailable, porcentajeUso, cpuUso, interfaces)
	if err != nil {
		return fmt.Errorf("error al guardar log: %w", err)
	}

	fmt.Println("Datos guardados en", monitor.RutaLogPrincipal)
	return nil
}

// ejecutarDaemon corre recolectarDatos() en bucle infinito, cada 5 minutos
func ejecutarDaemon() {
	fmt.Println("Modo daemon activado. Recolectando datos cada 5 minutos. (Ctrl+C para detener)")

	for {
		err := recolectarDatos()
		if err != nil {
			fmt.Println("Error durante la recolección:", err)
		}
		time.Sleep(5 * time.Minute)
	}
}
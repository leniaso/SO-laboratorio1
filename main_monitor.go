package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"system-monitor/monitor"
)

const rutaPIDDaemon = "system_monitor_logs/daemon.pid"
const rutaLogDaemon = "system_monitor_logs/daemon_output.log"
const rutaAlertsLog = "system_monitor_logs/alerts.log"

// nombresBinarios: monitor_app y alert_app se compilan por separado a partir de
// advanced_system_monitor.go y alert_system.go (cada uno es su propio "package main").
// main_monitor.go los invoca como procesos externos en vez de importarlos.
const binarioMonitor = "./monitor_app"
const binarioAlertas = "./alert_app"

var lector = bufio.NewReader(os.Stdin)

func main() {
	modoDaemon := flag.Bool("daemon", false, "Iniciar el daemon de monitoreo en segundo plano")
	modoReport := flag.Bool("report", false, "Generar el reporte diario (texto, csv y html)")
	modoAlert := flag.Bool("alert", false, "Verificar alertas inmediatamente")
	modoConfig := flag.Bool("config", false, "Editar la configuración de umbrales de alerta")
	flag.Parse()

	if err := os.MkdirAll("system_monitor_logs", 0755); err != nil {
		fmt.Println("Error creando directorio de logs:", err)
		return
	}

	switch {
	case *modoDaemon:
		iniciarDaemon()
	case *modoReport:
		generarReporteRapido()
	case *modoAlert:
		verificarAlertasAhora()
	case *modoConfig:
		configurarUmbrales()
	default:
		menuInteractivo()
	}
}

// ---------------------------------------------------------------------------
// Menú interactivo
// ---------------------------------------------------------------------------

func menuInteractivo() {
	for {
		fmt.Println("\n========================================")
		fmt.Println("   SYSTEM MONITOR - Menú principal")
		fmt.Println("========================================")
		fmt.Println("1. Iniciar daemon de monitoreo")
		fmt.Println("2. Detener daemon de monitoreo")
		fmt.Println("3. Ver estadísticas en tiempo real")
		fmt.Println("4. Generar reporte (diario/semanal/personalizado)")
		fmt.Println("5. Configurar umbrales de alertas")
		fmt.Println("6. Ver historial de alertas")
		fmt.Println("7. Verificar alertas ahora")
		fmt.Println("0. Salir")
		fmt.Print("Elige una opción: ")

		opcion := leerLinea()

		switch opcion {
		case "1":
			iniciarDaemon()
		case "2":
			detenerDaemon()
		case "3":
			verEstadisticasTiempoReal()
		case "4":
			generarReporteInteractivo()
		case "5":
			configurarUmbrales()
		case "6":
			verHistorialAlertas()
		case "7":
			verificarAlertasAhora()
		case "0":
			fmt.Println("Hasta luego 👋")
			return
		default:
			fmt.Println("Opción no válida, intenta de nuevo.")
		}
	}
}

func leerLinea() string {
	texto, _ := lector.ReadString('\n')
	return strings.TrimSpace(texto)
}

// ---------------------------------------------------------------------------
// Control del daemon (monitor_app --daemon) como proceso en segundo plano
// ---------------------------------------------------------------------------

func iniciarDaemon() {
	if pid, corriendo := daemonCorriendo(); corriendo {
		fmt.Printf("El daemon ya está corriendo (PID %d).\n", pid)
		return
	}

	if !binarioExiste(binarioMonitor) {
		mostrarInstruccionesBuild("monitor_app", "advanced_system_monitor.go")
		return
	}

	logFile, err := os.OpenFile(rutaLogDaemon, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error abriendo log del daemon:", err)
		return
	}

	cmd := exec.Command(binarioMonitor, "--daemon")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		fmt.Println("Error iniciando el daemon:", err)
		logFile.Close()
		return
	}

	err = os.WriteFile(rutaPIDDaemon, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
	if err != nil {
		fmt.Println("Aviso: el daemon inició pero no se pudo guardar el PID:", err)
	}

	fmt.Printf("Daemon iniciado en segundo plano (PID %d). Logs en %s\n", cmd.Process.Pid, rutaLogDaemon)
}

func detenerDaemon() {
	pid, corriendo := daemonCorriendo()
	if !corriendo {
		fmt.Println("El daemon no está corriendo (o no fue iniciado desde main_monitor).")
		return
	}

	proceso, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Error encontrando el proceso:", err)
		return
	}

	if err := proceso.Signal(syscall.SIGTERM); err != nil {
		fmt.Println("Error deteniendo el daemon:", err)
		return
	}

	os.Remove(rutaPIDDaemon)
	fmt.Printf("Daemon detenido (PID %d).\n", pid)
}

// daemonCorriendo revisa el archivo de PID y comprueba si ese proceso sigue vivo
func daemonCorriendo() (int, bool) {
	datos, err := os.ReadFile(rutaPIDDaemon)
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(datos)))
	if err != nil {
		return 0, false
	}

	proceso, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}

	// en Linux, FindProcess siempre tiene éxito; hay que enviar señal 0 para comprobar que existe
	if err := proceso.Signal(syscall.Signal(0)); err != nil {
		os.Remove(rutaPIDDaemon)
		return 0, false
	}

	return pid, true
}

// ---------------------------------------------------------------------------
// Estadísticas en tiempo real (usa el paquete monitor directamente, sin exec)
// ---------------------------------------------------------------------------

func verEstadisticasTiempoReal() {
	fmt.Println("\n-- Estadísticas en tiempo real --")

	memTotal, memAvailable, err := monitor.ObtenerMemoria()
	if err == nil {
		porcentaje := (float64(memTotal-memAvailable) / float64(memTotal)) * 100
		fmt.Printf("Memoria: %.2f%% en uso (%d kB / %d kB)\n", porcentaje, memTotal-memAvailable, memTotal)
	} else {
		fmt.Println("Error obteniendo memoria:", err)
	}

	fmt.Println("Calculando uso de CPU (toma ~0.5s)...")
	cpuUso, err := monitor.ObtenerUsoCPU()
	if err == nil {
		fmt.Printf("CPU: %.2f%% en uso\n", cpuUso)
	} else {
		fmt.Println("Error obteniendo CPU:", err)
	}

	load1, load5, load15, err := monitor.ObtenerCargaPromedio()
	if err == nil {
		fmt.Printf("Load average: %.2f (1m) | %.2f (5m) | %.2f (15m)\n", load1, load5, load15)
	} else {
		fmt.Println("Error obteniendo load average:", err)
	}

	usoDisco, err := monitor.ObtenerUsoDisco("/")
	if err == nil {
		fmt.Printf("Disco (/): %.2f%% en uso\n", usoDisco)
	} else {
		fmt.Println("Error obteniendo uso de disco:", err)
	}

	interfaces, err := monitor.ObtenerInterfacesRed()
	if err == nil {
		fmt.Println("Interfaces de red:")
		for _, iface := range interfaces {
			fmt.Printf("  %-8s RX: %d bytes | TX: %d bytes | Errores: %d | Drops: %d\n",
				iface.Nombre, iface.BytesRecibidos, iface.BytesEnviados, iface.ErroresRecibidos, iface.DropsRecibidos)
		}
	} else {
		fmt.Println("Error obteniendo interfaces de red:", err)
	}

	conexiones, err := monitor.ObtenerConexionesActivas()
	if err == nil {
		fmt.Printf("Conexiones: %d total | %d establecidas | %d en escucha\n",
			conexiones.Total, conexiones.Establecidas, conexiones.Escuchando)
	} else {
		fmt.Println("Error obteniendo conexiones activas:", err)
	}
}

// ---------------------------------------------------------------------------
// Generación de reportes (usa el paquete monitor directamente, sin exec)
// ---------------------------------------------------------------------------

// generarReporteRapido se usa con el flag --report: genera el reporte diario completo
func generarReporteRapido() {
	generarYGuardarReporte(24*time.Hour, true, true, true)
}

// generarReporteInteractivo se usa desde el menú, dejando elegir el periodo y el formato
func generarReporteInteractivo() {
	fmt.Println("\n¿Qué periodo quieres analizar?")
	fmt.Println("1. Diario (últimas 24h)")
	fmt.Println("2. Semanal (últimos 7 días)")
	fmt.Println("3. Personalizado (ej: 48h, 30m, 72h30m)")
	fmt.Print("Elige una opción: ")

	var duracion time.Duration
	switch leerLinea() {
	case "1":
		duracion = 24 * time.Hour
	case "2":
		duracion = 7 * 24 * time.Hour
	case "3":
		fmt.Print("Escribe la duración (formato Go, ej '48h'): ")
		d, err := time.ParseDuration(leerLinea())
		if err != nil {
			fmt.Println("Duración inválida:", err)
			return
		}
		duracion = d
	default:
		fmt.Println("Opción no válida.")
		return
	}

	fmt.Println("\n¿En qué formato(s)? (puedes elegir varios separados por espacio: texto csv html, o 'todos')")
	seleccion := strings.Fields(strings.ToLower(leerLinea()))
	if len(seleccion) == 0 {
		seleccion = []string{"todos"}
	}

	quiereTexto, quiereCSV, quiereHTML := false, false, false
	for _, s := range seleccion {
		switch s {
		case "texto", "txt":
			quiereTexto = true
		case "csv":
			quiereCSV = true
		case "html":
			quiereHTML = true
		case "todos":
			quiereTexto, quiereCSV, quiereHTML = true, true, true
		}
	}

	generarYGuardarReporte(duracion, quiereTexto, quiereCSV, quiereHTML)
}

func generarYGuardarReporte(duracion time.Duration, texto, csv, html bool) {
	fmt.Printf("Generando reporte (últimas %s)...\n", duracion)

	resumen, err := monitor.GenerarResumenReporte(duracion)
	if err != nil {
		fmt.Println("Error generando el resumen del reporte:", err)
		return
	}

	if texto {
		if ruta, err := monitor.GenerarReporteTexto(resumen); err != nil {
			fmt.Println("Error generando reporte de texto:", err)
		} else {
			fmt.Println("✅ Reporte de texto:", ruta)
		}
	}
	if csv {
		if ruta, err := monitor.GenerarReporteCSV(resumen); err != nil {
			fmt.Println("Error generando reporte CSV:", err)
		} else {
			fmt.Println("✅ Reporte CSV:", ruta)
		}
	}
	if html {
		if ruta, err := monitor.GenerarReporteHTML(resumen); err != nil {
			fmt.Println("Error generando reporte HTML:", err)
		} else {
			fmt.Println("✅ Reporte HTML:", ruta)
		}
	}
}

// ---------------------------------------------------------------------------
// Configuración de umbrales de alerta
// ---------------------------------------------------------------------------

func configurarUmbrales() {
	config, err := monitor.LeerConfig()
	if err != nil {
		fmt.Println("Aviso: no se pudo leer la configuración actual, se usarán valores por defecto:", err)
	}

	fmt.Println("\n-- Configuración actual de umbrales --")
	fmt.Printf("1. Umbral de RAM:               %.2f%%\n", config.UmbralRAMPercent)
	fmt.Printf("2. Umbral de load average:      %.2f\n", config.UmbralLoadCPU)
	fmt.Printf("3. Comprobaciones consecutivas: %d\n", config.ChequesConsecutivos)
	fmt.Printf("4. Umbral de disco:             %.2f%%\n", config.UmbralDiscoPercent)
	fmt.Println("\nPresiona Enter para mantener el valor actual de cada umbral.")

	config.UmbralRAMPercent = pedirFloat("Nuevo umbral de RAM (%)", config.UmbralRAMPercent)
	config.UmbralLoadCPU = pedirFloat("Nuevo umbral de load average", config.UmbralLoadCPU)
	config.ChequesConsecutivos = pedirInt("Nuevas comprobaciones consecutivas", config.ChequesConsecutivos)
	config.UmbralDiscoPercent = pedirFloat("Nuevo umbral de disco (%)", config.UmbralDiscoPercent)

	if err := monitor.GuardarConfig(config); err != nil {
		fmt.Println("Error guardando la configuración:", err)
		return
	}

	fmt.Println("✅ Configuración guardada en", monitor.RutaConfig)
	fmt.Println("(alert_app la leerá automáticamente en la próxima comprobación)")
}

func pedirFloat(mensaje string, actual float64) float64 {
	fmt.Printf("%s [%.2f]: ", mensaje, actual)
	entrada := leerLinea()
	if entrada == "" {
		return actual
	}
	valor, err := strconv.ParseFloat(entrada, 64)
	if err != nil {
		fmt.Println("Valor inválido, se mantiene el actual.")
		return actual
	}
	return valor
}

func pedirInt(mensaje string, actual int) int {
	fmt.Printf("%s [%d]: ", mensaje, actual)
	entrada := leerLinea()
	if entrada == "" {
		return actual
	}
	valor, err := strconv.Atoi(entrada)
	if err != nil {
		fmt.Println("Valor inválido, se mantiene el actual.")
		return actual
	}
	return valor
}

// ---------------------------------------------------------------------------
// Alertas: verificación inmediata e historial
// ---------------------------------------------------------------------------

func verificarAlertasAhora() {
	if !binarioExiste(binarioAlertas) {
		mostrarInstruccionesBuild("alert_app", "alert_system.go")
		return
	}

	fmt.Println("Verificando alertas...")
	cmd := exec.Command(binarioAlertas)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("Error ejecutando alert_app:", err)
	}
}

func verHistorialAlertas() {
	datos, err := os.ReadFile(rutaAlertsLog)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Todavía no hay alertas registradas.")
			return
		}
		fmt.Println("Error leyendo el historial de alertas:", err)
		return
	}

	lineas := strings.Split(strings.TrimRight(string(datos), "\n"), "\n")
	if len(lineas) == 0 || (len(lineas) == 1 && lineas[0] == "") {
		fmt.Println("Todavía no hay alertas registradas.")
		return
	}

	const maxLineas = 20
	inicio := 0
	if len(lineas) > maxLineas {
		inicio = len(lineas) - maxLineas
		fmt.Printf("Mostrando las últimas %d alertas de %d:\n\n", maxLineas, len(lineas))
	} else {
		fmt.Println("\n-- Historial de alertas --")
	}

	for _, linea := range lineas[inicio:] {
		fmt.Println(linea)
	}
}

// ---------------------------------------------------------------------------
// Utilidades
// ---------------------------------------------------------------------------

func binarioExiste(ruta string) bool {
	info, err := os.Stat(ruta)
	return err == nil && !info.IsDir()
}

func mostrarInstruccionesBuild(nombreBinario, archivoFuente string) {
	fmt.Printf("No se encontró %s. Compílalo primero con:\n", nombreBinario)
	fmt.Printf("  go build -o %s %s\n", nombreBinario, archivoFuente)
}

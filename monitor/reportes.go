package monitor

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// EntradaLogRed representa únicamente los campos de red de una fila del monitor_log.csv
type EntradaLogRed struct {
	Timestamp      time.Time
	RxBytesTotal   int64
	TxBytesTotal   int64
	ErroresRxTotal int64
	DropsRxTotal   int64
}

// ProcesoPromedio agrupa el uso promedio de CPU/memoria de un proceso a lo largo del historial
type ProcesoPromedio struct {
	Nombre      string
	CPUPromedio float64
	MemPromedio float64
	Apariciones int
}

// ReporteResumen contiene todos los datos ya calculados que necesita un reporte
type ReporteResumen struct {
	GeneradoEn        time.Time
	Periodo           time.Duration
	Uptime            string
	Load1             float64
	Load5             float64
	Load15            float64
	PicoMemoria       float64
	PicoMemoriaFecha  time.Time
	PromedioMemoria   float64
	PromedioCPU       float64
	TopProcesos       []ProcesoPromedio
	RxTotalMB         float64
	TxTotalMB         float64
	ErroresRxTotal    int64
	DropsRxTotal      int64
	CantidadRegistros int
}

// Rutas de salida de los reportes. %s se reemplaza por la fecha (YYYY-MM-DD)
const RutaReporteTxt = "system_monitor_logs/reporte_%s.txt"
const RutaReporteCSV = "system_monitor_logs/reporte_%s.csv"
const RutaReporteHTML = "system_monitor_logs/reporte_%s.html"

// ObtenerUptime lee /proc/uptime y lo devuelve formateado como "X días, Y horas, Z minutos"
func ObtenerUptime() (string, error) {
	datos, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}

	campos := strings.Fields(string(datos))
	if len(campos) < 1 {
		return "", fmt.Errorf("formato inesperado en /proc/uptime")
	}

	segundosTotales, err := strconv.ParseFloat(campos[0], 64)
	if err != nil {
		return "", err
	}

	duracion := time.Duration(segundosTotales) * time.Second
	dias := int(duracion.Hours()) / 24
	horas := int(duracion.Hours()) % 24
	minutos := int(duracion.Minutes()) % 60

	return fmt.Sprintf("%d días, %d horas, %d minutos", dias, horas, minutos), nil
}

// LeerLogRedCompleto lee monitor_log.csv y extrae solo las columnas relacionadas con red
func LeerLogRedCompleto() ([]EntradaLogRed, error) {
	file, err := os.Open(RutaLogPrincipal)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	filas, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var entradas []EntradaLogRed

	for i, fila := range filas {
		if i == 0 {
			continue
		}
		if len(fila) < 9 {
			continue
		}

		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", fila[0], time.Local)
		if err != nil {
			continue
		}

		rx, err1 := strconv.ParseInt(fila[5], 10, 64)
		tx, err2 := strconv.ParseInt(fila[6], 10, 64)
		errRx, err3 := strconv.ParseInt(fila[7], 10, 64)
		dropsRx, err4 := strconv.ParseInt(fila[8], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}

		entradas = append(entradas, EntradaLogRed{
			Timestamp:      timestamp,
			RxBytesTotal:   rx,
			TxBytesTotal:   tx,
			ErroresRxTotal: errRx,
			DropsRxTotal:   dropsRx,
		})
	}

	return entradas, nil
}

// CalcularPicoMemoria encuentra el valor máximo de uso de memoria y el momento en que ocurrió
func CalcularPicoMemoria(entradas []EntradaLog) (float64, time.Time, error) {
	if len(entradas) == 0 {
		return 0, time.Time{}, fmt.Errorf("no hay datos para calcular el pico de memoria")
	}

	pico := entradas[0].MemUsadaPercent
	fecha := entradas[0].Timestamp

	for _, e := range entradas {
		if e.MemUsadaPercent > pico {
			pico = e.MemUsadaPercent
			fecha = e.Timestamp
		}
	}

	return pico, fecha, nil
}

// CalcularTraficoRed calcula el tráfico total (en MB) dentro de una ventana de tiempo,
// comparando el primer y el último registro que caen dentro de esa ventana. Como los
// contadores de /proc/net/dev son acumulativos desde el arranque del sistema, la resta
// entre el último y el primer registro nos da el tráfico generado en el periodo.
func CalcularTraficoRed(entradas []EntradaLogRed, duracion time.Duration) (rxMB, txMB float64, erroresRx, dropsRx int64, err error) {
	limite := time.Now().Add(-duracion)

	var filtradas []EntradaLogRed
	for _, e := range entradas {
		if e.Timestamp.After(limite) {
			filtradas = append(filtradas, e)
		}
	}

	if len(filtradas) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no hay datos de red suficientes en el periodo solicitado")
	}

	sort.Slice(filtradas, func(i, j int) bool {
		return filtradas[i].Timestamp.Before(filtradas[j].Timestamp)
	})

	primero := filtradas[0]
	ultimo := filtradas[len(filtradas)-1]

	deltaRx := ultimo.RxBytesTotal - primero.RxBytesTotal
	deltaTx := ultimo.TxBytesTotal - primero.TxBytesTotal

	// si el contador se reinició (ej. reinicio de la interfaz), evitamos un delta negativo
	if deltaRx < 0 {
		deltaRx = ultimo.RxBytesTotal
	}
	if deltaTx < 0 {
		deltaTx = ultimo.TxBytesTotal
	}

	rxMB = float64(deltaRx) / (1024 * 1024)
	txMB = float64(deltaTx) / (1024 * 1024)
	erroresRx = ultimo.ErroresRxTotal
	dropsRx = ultimo.DropsRxTotal

	return rxMB, txMB, erroresRx, dropsRx, nil
}

// CalcularTopProcesosPromedio agrupa procesos_log.csv por nombre de proceso y calcula
// el promedio histórico de CPU/memoria, devolviendo los N con mayor CPU promedio
func CalcularTopProcesosPromedio(n int) ([]ProcesoPromedio, error) {
	file, err := os.Open(RutaLogProcesos)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	filas, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	type acumulado struct {
		sumaCPU float64
		sumaMem float64
		veces   int
	}
	acumulados := make(map[string]*acumulado)

	for i, fila := range filas {
		if i == 0 || len(fila) < 5 {
			continue
		}

		nombre := fila[2]
		cpu, err1 := strconv.ParseFloat(fila[3], 64)
		mem, err2 := strconv.ParseFloat(fila[4], 64)
		if err1 != nil || err2 != nil {
			continue
		}

		if acumulados[nombre] == nil {
			acumulados[nombre] = &acumulado{}
		}
		acumulados[nombre].sumaCPU += cpu
		acumulados[nombre].sumaMem += mem
		acumulados[nombre].veces++
	}

	var resultado []ProcesoPromedio
	for nombre, a := range acumulados {
		resultado = append(resultado, ProcesoPromedio{
			Nombre:      nombre,
			CPUPromedio: a.sumaCPU / float64(a.veces),
			MemPromedio: a.sumaMem / float64(a.veces),
			Apariciones: a.veces,
		})
	}

	sort.Slice(resultado, func(i, j int) bool {
		return resultado[i].CPUPromedio > resultado[j].CPUPromedio
	})

	if len(resultado) > n {
		resultado = resultado[:n]
	}

	return resultado, nil
}

// GenerarResumenReporte reúne todos los cálculos anteriores en un solo ReporteResumen,
// filtrando el historial a la ventana de tiempo indicada por `duracion`
func GenerarResumenReporte(duracion time.Duration) (ReporteResumen, error) {
	var resumen ReporteResumen
	resumen.GeneradoEn = time.Now()
	resumen.Periodo = duracion

	if uptime, err := ObtenerUptime(); err == nil {
		resumen.Uptime = uptime
	}

	if load1, load5, load15, err := ObtenerCargaPromedio(); err == nil {
		resumen.Load1, resumen.Load5, resumen.Load15 = load1, load5, load15
	}

	entradas, err := LeerLogCompleto()
	if err != nil {
		return resumen, fmt.Errorf("error leyendo el log principal (¿ya corriste el monitor alguna vez?): %w", err)
	}

	limite := time.Now().Add(-duracion)
	var entradasPeriodo []EntradaLog
	for _, e := range entradas {
		if e.Timestamp.After(limite) {
			entradasPeriodo = append(entradasPeriodo, e)
		}
	}
	resumen.CantidadRegistros = len(entradasPeriodo)

	if len(entradasPeriodo) > 0 {
		if pico, fecha, err := CalcularPicoMemoria(entradasPeriodo); err == nil {
			resumen.PicoMemoria = pico
			resumen.PicoMemoriaFecha = fecha
		}
		if promMem, err := CalcularPromedioMemoria(entradasPeriodo, duracion); err == nil {
			resumen.PromedioMemoria = promMem
		}
		if promCPU, err := CalcularPromedioCPU(entradasPeriodo, duracion); err == nil {
			resumen.PromedioCPU = promCPU
		}
	}

	if topProcesos, err := CalcularTopProcesosPromedio(5); err == nil {
		resumen.TopProcesos = topProcesos
	}

	if entradasRed, err := LeerLogRedCompleto(); err == nil {
		if rxMB, txMB, erroresRx, dropsRx, err := CalcularTraficoRed(entradasRed, duracion); err == nil {
			resumen.RxTotalMB = rxMB
			resumen.TxTotalMB = txMB
			resumen.ErroresRxTotal = erroresRx
			resumen.DropsRxTotal = dropsRx
		}
	}

	return resumen, nil
}

// GenerarReporteTexto escribe un reporte legible en texto plano y devuelve la ruta generada
func GenerarReporteTexto(resumen ReporteResumen) (string, error) {
	fecha := resumen.GeneradoEn.Format("2006-01-02")
	ruta := fmt.Sprintf(RutaReporteTxt, fecha)

	var sb strings.Builder

	sb.WriteString("========================================\n")
	sb.WriteString("   REPORTE DE SISTEMA\n")
	fmt.Fprintf(&sb, "   Generado: %s\n", resumen.GeneradoEn.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "   Periodo analizado: últimas %s\n", resumen.Periodo)
	sb.WriteString("========================================\n\n")

	fmt.Fprintf(&sb, "Uptime del sistema: %s\n", resumen.Uptime)
	fmt.Fprintf(&sb, "Load average: %.2f (1m) | %.2f (5m) | %.2f (15m)\n\n", resumen.Load1, resumen.Load5, resumen.Load15)

	sb.WriteString("-- MEMORIA --\n")
	if resumen.CantidadRegistros == 0 {
		sb.WriteString("(sin registros en el historial para este periodo; corre 'monitor_app --daemon' o 'monitor_app' primero)\n\n")
	} else {
		fmt.Fprintf(&sb, "Pico de uso: %.2f%% el %s\n", resumen.PicoMemoria, resumen.PicoMemoriaFecha.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&sb, "Promedio del periodo: %.2f%%\n\n", resumen.PromedioMemoria)
	}

	sb.WriteString("-- CPU --\n")
	fmt.Fprintf(&sb, "Promedio del periodo: %.2f%%\n\n", resumen.PromedioCPU)

	sb.WriteString("-- TOP 5 PROCESOS POR CPU PROMEDIO --\n")
	if len(resumen.TopProcesos) == 0 {
		sb.WriteString("(sin datos suficientes; corre 'monitor_app --procesos' varias veces para acumular historial)\n\n")
	} else {
		for i, p := range resumen.TopProcesos {
			fmt.Fprintf(&sb, "%d. %-20s CPU: %5.2f%% | MEM: %5.2f%% | %d muestras\n",
				i+1, p.Nombre, p.CPUPromedio, p.MemPromedio, p.Apariciones)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("-- RED --\n")
	fmt.Fprintf(&sb, "Tráfico recibido:   %.2f MB\n", resumen.RxTotalMB)
	fmt.Fprintf(&sb, "Tráfico enviado:    %.2f MB\n", resumen.TxTotalMB)
	fmt.Fprintf(&sb, "Errores RX totales: %d | Drops RX totales: %d\n\n", resumen.ErroresRxTotal, resumen.DropsRxTotal)

	fmt.Fprintf(&sb, "(Basado en %d registros del historial)\n", resumen.CantidadRegistros)

	err := os.WriteFile(ruta, []byte(sb.String()), 0644)
	return ruta, err
}

// GenerarReporteCSV escribe un reporte en formato CSV (clave,valor + tabla de procesos)
func GenerarReporteCSV(resumen ReporteResumen) (string, error) {
	fecha := resumen.GeneradoEn.Format("2006-01-02")
	ruta := fmt.Sprintf(RutaReporteCSV, fecha)

	file, err := os.Create(ruta)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	filas := [][]string{
		{"metrica", "valor"},
		{"generado_en", resumen.GeneradoEn.Format("2006-01-02 15:04:05")},
		{"periodo", resumen.Periodo.String()},
		{"uptime", resumen.Uptime},
		{"load1", fmt.Sprintf("%.2f", resumen.Load1)},
		{"load5", fmt.Sprintf("%.2f", resumen.Load5)},
		{"load15", fmt.Sprintf("%.2f", resumen.Load15)},
		{"pico_memoria_percent", fmt.Sprintf("%.2f", resumen.PicoMemoria)},
		{"pico_memoria_fecha", resumen.PicoMemoriaFecha.Format("2006-01-02 15:04:05")},
		{"promedio_memoria_percent", fmt.Sprintf("%.2f", resumen.PromedioMemoria)},
		{"promedio_cpu_percent", fmt.Sprintf("%.2f", resumen.PromedioCPU)},
		{"trafico_rx_mb", fmt.Sprintf("%.2f", resumen.RxTotalMB)},
		{"trafico_tx_mb", fmt.Sprintf("%.2f", resumen.TxTotalMB)},
		{"errores_rx_total", fmt.Sprintf("%d", resumen.ErroresRxTotal)},
		{"drops_rx_total", fmt.Sprintf("%d", resumen.DropsRxTotal)},
		{"registros_analizados", fmt.Sprintf("%d", resumen.CantidadRegistros)},
		{},
		{"top_procesos_nombre", "cpu_promedio", "mem_promedio", "apariciones"},
	}

	for _, fila := range filas {
		if err := writer.Write(fila); err != nil {
			return "", err
		}
	}

	for _, p := range resumen.TopProcesos {
		fila := []string{p.Nombre, fmt.Sprintf("%.2f", p.CPUPromedio), fmt.Sprintf("%.2f", p.MemPromedio), fmt.Sprintf("%d", p.Apariciones)}
		if err := writer.Write(fila); err != nil {
			return "", err
		}
	}

	return ruta, nil
}

// GenerarReporteHTML escribe un reporte en HTML con estilos básicos (tarea bonus)
func GenerarReporteHTML(resumen ReporteResumen) (string, error) {
	fecha := resumen.GeneradoEn.Format("2006-01-02")
	ruta := fmt.Sprintf(RutaReporteHTML, fecha)

	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html lang=\"es\">\n<head>\n<meta charset=\"UTF-8\">\n")
	fmt.Fprintf(&sb, "<title>Reporte del sistema - %s</title>\n", fecha)
	sb.WriteString(`<style>
  body { font-family: Arial, sans-serif; background: #f4f6f8; color: #222; margin: 0; padding: 2rem; }
  h1 { color: #1a3d5c; }
  .card { background: white; border-radius: 8px; padding: 1.2rem 1.5rem; margin-bottom: 1rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .metric { display: flex; justify-content: space-between; padding: 0.3rem 0; border-bottom: 1px solid #eee; }
  table { width: 100%; border-collapse: collapse; margin-top: 0.5rem; }
  th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #eee; }
  th { background: #1a3d5c; color: white; }
</style>
</head>
<body>
`)
	sb.WriteString("  <h1>Reporte del sistema</h1>\n")
	fmt.Fprintf(&sb, "  <p>Generado el %s &mdash; periodo analizado: últimas %s</p>\n\n",
		resumen.GeneradoEn.Format("2006-01-02 15:04:05"), resumen.Periodo)

	sb.WriteString("  <div class=\"card\">\n    <h2>Resumen general</h2>\n")
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Uptime</span><span>%s</span></div>\n", resumen.Uptime)
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Load average (1m / 5m / 15m)</span><span>%.2f / %.2f / %.2f</span></div>\n",
		resumen.Load1, resumen.Load5, resumen.Load15)
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Pico de memoria</span><span>%.2f%% (%s)</span></div>\n",
		resumen.PicoMemoria, resumen.PicoMemoriaFecha.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Promedio de memoria</span><span>%.2f%%</span></div>\n", resumen.PromedioMemoria)
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Promedio de CPU</span><span>%.2f%%</span></div>\n", resumen.PromedioCPU)
	sb.WriteString("  </div>\n\n")

	sb.WriteString("  <div class=\"card\">\n    <h2>Red</h2>\n")
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Tráfico recibido</span><span>%.2f MB</span></div>\n", resumen.RxTotalMB)
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Tráfico enviado</span><span>%.2f MB</span></div>\n", resumen.TxTotalMB)
	fmt.Fprintf(&sb, "    <div class=\"metric\"><span>Errores / Drops RX</span><span>%d / %d</span></div>\n",
		resumen.ErroresRxTotal, resumen.DropsRxTotal)
	sb.WriteString("  </div>\n\n")

	sb.WriteString("  <div class=\"card\">\n    <h2>Top 5 procesos por CPU promedio</h2>\n    <table>\n")
	sb.WriteString("      <tr><th>#</th><th>Proceso</th><th>CPU prom.</th><th>Mem prom.</th></tr>\n")
	if len(resumen.TopProcesos) == 0 {
		sb.WriteString("      <tr><td colspan=\"4\">Sin datos suficientes</td></tr>\n")
	} else {
		for i, p := range resumen.TopProcesos {
			fmt.Fprintf(&sb, "      <tr><td>%d</td><td>%s</td><td>%.2f%%</td><td>%.2f%%</td></tr>\n",
				i+1, p.Nombre, p.CPUPromedio, p.MemPromedio)
		}
	}
	sb.WriteString("    </table>\n  </div>\n\n")

	fmt.Fprintf(&sb, "  <p style=\"color:#888; font-size: 0.85rem;\">Basado en %d registros del historial.</p>\n", resumen.CantidadRegistros)
	sb.WriteString("</body>\n</html>\n")

	err := os.WriteFile(ruta, []byte(sb.String()), 0644)
	return ruta, err
}

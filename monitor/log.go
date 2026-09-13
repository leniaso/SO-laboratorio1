package monitor

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// EntradaLog representa una fila ya parseada del monitor_log.csv
type EntradaLog struct {
	Timestamp       time.Time
	MemUsadaPercent float64
	CPUPercent      float64
}

// ResultadoFuga contiene el resultado del análisis de posible fuga de memoria
type ResultadoFuga struct {
	HayFuga      bool
	MemActual    float64
	PromedioHora float64
	Diferencia   float64
}

// ProcesoInfo representa un proceso con su uso de CPU y memoria
type ProcesoInfo struct {
	PID    string
	Nombre string
	CPU    float64
	Mem    float64
}

const RutaLogPrincipal = "system_monitor_logs/monitor_log.csv"
const RutaLogProcesos = "system_monitor_logs/procesos_log.csv"

// GuardarLogCSV escribe una línea con los datos recolectados en el archivo CSV
func GuardarLogCSV(memTotal, memAvailable int, porcentajeMem float64, cpuUso float64, interfaces []InterfaceRed) error {
	_, err := os.Stat(RutaLogPrincipal)
	archivoExiste := err == nil

	file, err := os.OpenFile(RutaLogPrincipal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if !archivoExiste {
		encabezado := "timestamp,mem_total_kb,mem_available_kb,mem_usada_percent,cpu_percent,rx_bytes_total,tx_bytes_total,errores_rx_total,drops_rx_total\n"
		_, err = file.WriteString(encabezado)
		if err != nil {
			return err
		}
	}

	rxTotal, txTotal, erroresRxTotal, dropsRxTotal := CalcularTotalesRed(interfaces)

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	linea := fmt.Sprintf("%s,%d,%d,%.2f,%.2f,%d,%d,%d,%d\n",
		timestamp, memTotal, memAvailable, porcentajeMem, cpuUso, rxTotal, txTotal, erroresRxTotal, dropsRxTotal)

	_, err = file.WriteString(linea)
	if err != nil {
		return err
	}

	return nil
}

// LeerLogCompleto lee todo el monitor_log.csv y lo convierte en un slice de EntradaLog
func LeerLogCompleto() ([]EntradaLog, error) {
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

	var entradas []EntradaLog

	for i, fila := range filas {
		if i == 0 {
			continue
		}
		if len(fila) < 5 {
			continue
		}

		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", fila[0], time.Local)
		if err != nil {
			continue
		}

		memPercent, err := strconv.ParseFloat(fila[3], 64)
		if err != nil {
			continue
		}

		cpuPercent, err := strconv.ParseFloat(fila[4], 64)
		if err != nil {
			continue
		}

		entradas = append(entradas, EntradaLog{
			Timestamp:       timestamp,
			MemUsadaPercent: memPercent,
			CPUPercent:      cpuPercent,
		})
	}

	return entradas, nil
}

// CalcularPromedioMemoria calcula el promedio de uso de memoria en las últimas `duracion`
func CalcularPromedioMemoria(entradas []EntradaLog, duracion time.Duration) (float64, error) {
	ahora := time.Now()
	limite := ahora.Add(-duracion)

	var suma float64
	var contador int

	for _, entrada := range entradas {
		if entrada.Timestamp.After(limite) {
			suma += entrada.MemUsadaPercent
			contador++
		}
	}

	if contador == 0 {
		return 0, fmt.Errorf("no hay datos suficientes en el periodo solicitado")
	}

	return suma / float64(contador), nil
}

// CalcularPromedioCPU calcula el promedio de uso de CPU en las últimas `duracion`
func CalcularPromedioCPU(entradas []EntradaLog, duracion time.Duration) (float64, error) {
	ahora := time.Now()
	limite := ahora.Add(-duracion)

	var suma float64
	var contador int

	for _, entrada := range entradas {
		if entrada.Timestamp.After(limite) {
			suma += entrada.CPUPercent
			contador++
		}
	}

	if contador == 0 {
		return 0, fmt.Errorf("no hay datos suficientes en el periodo solicitado")
	}

	return suma / float64(contador), nil
}

// DetectarFugaMemoria compara el uso actual de memoria contra el promedio de la última hora
func DetectarFugaMemoria(umbralPorcentaje float64) (ResultadoFuga, error) {
	memTotal, memAvailable, err := ObtenerMemoria()
	if err != nil {
		return ResultadoFuga{}, fmt.Errorf("error al obtener memoria actual: %w", err)
	}
	memActual := (float64(memTotal-memAvailable) / float64(memTotal)) * 100

	entradas, err := LeerLogCompleto()
	if err != nil {
		return ResultadoFuga{}, fmt.Errorf("error al leer el log: %w", err)
	}

	promedioHora, err := CalcularPromedioMemoria(entradas, 1*time.Hour)
	if err != nil {
		return ResultadoFuga{}, fmt.Errorf("error al calcular promedio: %w", err)
	}

	diferencia := memActual - promedioHora

	return ResultadoFuga{
		HayFuga:      diferencia > umbralPorcentaje,
		MemActual:    memActual,
		PromedioHora: promedioHora,
		Diferencia:   diferencia,
	}, nil
}

// ObtenerTopProcesos ejecuta `ps` y devuelve los top N procesos ordenados por CPU
func ObtenerTopProcesos(n int) ([]ProcesoInfo, error) {
	cmd := exec.Command("ps", "-eo", "pid,comm,%cpu,%mem", "--sort=-%cpu")

	salida, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error ejecutando ps: %w", err)
	}

	var procesos []ProcesoInfo

	scanner := bufio.NewScanner(strings.NewReader(string(salida)))
	numeroLinea := 0

	for scanner.Scan() {
		numeroLinea++
		if numeroLinea == 1 {
			continue
		}

		linea := strings.TrimSpace(scanner.Text())
		campos := strings.Fields(linea)

		if len(campos) < 4 {
			continue
		}

		cpu, err := strconv.ParseFloat(campos[len(campos)-2], 64)
		if err != nil {
			continue
		}
		mem, err := strconv.ParseFloat(campos[len(campos)-1], 64)
		if err != nil {
			continue
		}

		nombre := strings.Join(campos[1:len(campos)-2], " ")

		procesos = append(procesos, ProcesoInfo{
			PID:    campos[0],
			Nombre: nombre,
			CPU:    cpu,
			Mem:    mem,
		})

		if len(procesos) >= n {
			break
		}
	}

	return procesos, nil
}

// GuardarTopProcesosCSV guarda el snapshot actual de top procesos en su propio log
func GuardarTopProcesosCSV(procesos []ProcesoInfo) error {
	_, err := os.Stat(RutaLogProcesos)
	archivoExiste := err == nil

	file, err := os.OpenFile(RutaLogProcesos, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if !archivoExiste {
		_, err = file.WriteString("timestamp,pid,nombre,cpu_percent,mem_percent\n")
		if err != nil {
			return err
		}
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	for _, p := range procesos {
		linea := fmt.Sprintf("%s,%s,%s,%.2f,%.2f\n", timestamp, p.PID, p.Nombre, p.CPU, p.Mem)
		_, err = file.WriteString(linea)
		if err != nil {
			return err
		}
	}

	return nil
}

// ContarApariciones cuenta cuántas veces un proceso (por nombre) aparece en el historial
func ContarApariciones(nombreProceso string) (int, error) {
	file, err := os.Open(RutaLogProcesos)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	filas, err := reader.ReadAll()
	if err != nil {
		return 0, err
	}

	contador := 0
	for i, fila := range filas {
		if i == 0 || len(fila) < 3 {
			continue
		}
		if fila[2] == nombreProceso {
			contador++
		}
	}

	return contador, nil
}
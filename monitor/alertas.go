package monitor

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ObtenerCargaPromedio lee /proc/loadavg y devuelve el load average de 1, 5 y 15 minutos
func ObtenerCargaPromedio() (float64, float64, float64, error) {
	file, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	linea := scanner.Text()

	campos := strings.Fields(linea)
	if len(campos) < 3 {
		return 0, 0, 0, fmt.Errorf("formato inesperado en /proc/loadavg")
	}

	load1, err := strconv.ParseFloat(campos[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load5, err := strconv.ParseFloat(campos[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load15, err := strconv.ParseFloat(campos[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return load1, load5, load15, nil
}

// ObtenerUsoDisco devuelve el porcentaje de uso del disco para la ruta dada (ej: "/")
func ObtenerUsoDisco(ruta string) (float64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(ruta, &stat)
	if err != nil {
		return 0, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	libre := stat.Bfree * uint64(stat.Bsize)
	usado := total - libre

	porcentaje := (float64(usado) / float64(total)) * 100
	return porcentaje, nil
}
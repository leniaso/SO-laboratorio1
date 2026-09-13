package monitor

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// ObtenerMemoria lee /proc/meminfo y devuelve MemTotal y MemAvailable en kB
func ObtenerMemoria() (int, int, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var memTotal, memAvailable int
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = extraerNumero(line)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable = extraerNumero(line)
		}
	}

	return memTotal, memAvailable, nil
}

// extraerNumero toma una línea tipo "MemTotal:  16384000 kB" y devuelve 16384000
func extraerNumero(line string) int {
	campos := strings.Fields(line)
	numero, err := strconv.Atoi(campos[1])
	if err != nil {
		return 0
	}
	return numero
}
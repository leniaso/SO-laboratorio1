package monitor

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// leerStatCPU lee la primera línea de /proc/stat y devuelve (idle, total, error)
func leerStatCPU() (int, int, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	line := scanner.Text()

	campos := strings.Fields(line)

	var total int
	var idle int

	for i, campo := range campos {
		if i == 0 {
			continue
		}
		valor, err := strconv.Atoi(campo)
		if err != nil {
			continue
		}
		total += valor
		if i == 4 {
			idle = valor
		}
	}

	return idle, total, nil
}

// ObtenerUsoCPU hace dos lecturas separadas por 500ms y calcula el % de uso
func ObtenerUsoCPU() (float64, error) {
	idle1, total1, err := leerStatCPU()
	if err != nil {
		return 0, err
	}

	time.Sleep(500 * time.Millisecond)

	idle2, total2, err := leerStatCPU()
	if err != nil {
		return 0, err
	}

	deltaIdle := idle2 - idle1
	deltaTotal := total2 - total1

	uso := (1 - (float64(deltaIdle) / float64(deltaTotal))) * 100
	return uso, nil
}
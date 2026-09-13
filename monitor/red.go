package monitor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// InterfaceRed agrupa todos los datos de una interfaz de red
type InterfaceRed struct {
	Nombre           string
	BytesRecibidos   int
	BytesEnviados    int
	ErroresRecibidos int
	ErroresEnviados  int
	DropsRecibidos   int
	DropsEnviados    int
}

// ResumenConexiones agrupa el conteo de conexiones por estado
type ResumenConexiones struct {
	Establecidas int
	Escuchando   int
	Total        int
}

// ObtenerInterfacesRed lee /proc/net/dev y devuelve un slice con los datos de cada interfaz
func ObtenerInterfacesRed() ([]InterfaceRed, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var interfaces []InterfaceRed

	scanner := bufio.NewScanner(file)
	numeroLinea := 0

	for scanner.Scan() {
		numeroLinea++
		if numeroLinea <= 2 {
			continue
		}

		line := scanner.Text()
		partes := strings.SplitN(line, ":", 2)
		if len(partes) != 2 {
			continue
		}

		nombre := strings.TrimSpace(partes[0])
		datos := strings.Fields(partes[1])

		if len(datos) < 16 {
			continue
		}

		iface := InterfaceRed{
			Nombre:           nombre,
			BytesRecibidos:   atoiSeguro(datos[0]),
			ErroresRecibidos: atoiSeguro(datos[2]),
			DropsRecibidos:   atoiSeguro(datos[3]),
			BytesEnviados:    atoiSeguro(datos[8]),
			ErroresEnviados:  atoiSeguro(datos[10]),
			DropsEnviados:    atoiSeguro(datos[11]),
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// atoiSeguro convierte string a int, y devuelve 0 si falla
func atoiSeguro(s string) int {
	valor, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return valor
}

// CalcularTotalesRed suma bytes/errores/drops de todas las interfaces
func CalcularTotalesRed(interfaces []InterfaceRed) (rxTotal, txTotal, erroresRxTotal, dropsRxTotal int) {
	for _, iface := range interfaces {
		rxTotal += iface.BytesRecibidos
		txTotal += iface.BytesEnviados
		erroresRxTotal += iface.ErroresRecibidos
		dropsRxTotal += iface.DropsRecibidos
	}
	return
}

// ObtenerConexionesActivas ejecuta `ss -tan` y cuenta las conexiones por estado
func ObtenerConexionesActivas() (ResumenConexiones, error) {
	cmd := exec.Command("ss", "-tan")

	salida, err := cmd.Output()
	if err != nil {
		return ResumenConexiones{}, fmt.Errorf("error ejecutando ss: %w", err)
	}

	var resumen ResumenConexiones

	scanner := bufio.NewScanner(strings.NewReader(string(salida)))
	numeroLinea := 0

	for scanner.Scan() {
		numeroLinea++
		if numeroLinea == 1 {
			continue
		}

		linea := strings.TrimSpace(scanner.Text())
		campos := strings.Fields(linea)
		if len(campos) < 1 {
			continue
		}

		estado := campos[0]
		resumen.Total++

		switch estado {
		case "ESTAB":
			resumen.Establecidas++
		case "LISTEN":
			resumen.Escuchando++
		}
	}

	return resumen, nil
}

// InterfaceCaida verifica si alguna interfaz (que no sea loopback) está inactiva.
// Devuelve el nombre de la primera interfaz caída encontrada, o "" si todas están bien.
func InterfaceCaida() (string, error) {
	cmd := exec.Command("ip", "-o", "link", "show")
	salida, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error ejecutando ip link: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(salida)))
	for scanner.Scan() {
		linea := scanner.Text()

		if strings.Contains(linea, "lo:") {
			continue // ignoramos loopback, siempre está "up"
		}

		// las líneas de interfaces caídas contienen "state DOWN"
		if strings.Contains(linea, "state DOWN") {
			campos := strings.Fields(linea)
			if len(campos) >= 2 {
				nombre := strings.TrimSuffix(campos[1], ":")
				return nombre, nil
			}
		}
	}

	return "", nil // ninguna interfaz caída
}
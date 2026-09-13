package main

import (
	"flag"
	"fmt"
	"time"

	"system-monitor/monitor"
)

func main() {
	modoTexto := flag.Bool("texto", false, "Generar reporte en formato texto plano")
	modoCSV := flag.Bool("csv", false, "Generar reporte en formato CSV")
	modoHTML := flag.Bool("html", false, "Generar reporte en formato HTML")
	periodo := flag.String("periodo", "diario", "Periodo del reporte: 'diario', 'semanal', o duración personalizada como '48h'")
	flag.Parse()

	duracion, err := resolverPeriodo(*periodo)
	if err != nil {
		fmt.Println("Periodo inválido:", err)
		fmt.Println("Usa 'diario', 'semanal', o una duración estilo Go (ej: '48h', '72h30m')")
		return
	}

	fmt.Printf("Generando reporte del sistema (últimas %s)...\n", duracion)

	resumen, err := monitor.GenerarResumenReporte(duracion)
	if err != nil {
		fmt.Println("Error generando el resumen del reporte:", err)
		return
	}

	// Si no se especifica ningún formato explícito, generamos los tres
	generarTodos := !*modoTexto && !*modoCSV && !*modoHTML

	if *modoTexto || generarTodos {
		ruta, err := monitor.GenerarReporteTexto(resumen)
		if err != nil {
			fmt.Println("Error generando reporte de texto:", err)
		} else {
			fmt.Println("✅ Reporte de texto guardado en", ruta)
		}
	}

	if *modoCSV || generarTodos {
		ruta, err := monitor.GenerarReporteCSV(resumen)
		if err != nil {
			fmt.Println("Error generando reporte CSV:", err)
		} else {
			fmt.Println("✅ Reporte CSV guardado en", ruta)
		}
	}

	if *modoHTML || generarTodos {
		ruta, err := monitor.GenerarReporteHTML(resumen)
		if err != nil {
			fmt.Println("Error generando reporte HTML:", err)
		} else {
			fmt.Println("✅ Reporte HTML guardado en", ruta)
		}
	}
}

// resolverPeriodo traduce los atajos "diario"/"semanal" a time.Duration,
// o intenta parsear el string como una duración de Go (ej: "48h")
func resolverPeriodo(periodo string) (time.Duration, error) {
	switch periodo {
	case "diario":
		return 24 * time.Hour, nil
	case "semanal":
		return 7 * 24 * time.Hour, nil
	default:
		return time.ParseDuration(periodo)
	}
}

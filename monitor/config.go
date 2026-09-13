package monitor

import (
	"encoding/json"
	"os"
)

// ConfigAlertas contiene los umbrales configurables del sistema de alertas.
// Si no existe un archivo de configuración, se usan los valores del enunciado del laboratorio.
type ConfigAlertas struct {
	UmbralRAMPercent    float64 `json:"umbral_ram_percent"`
	UmbralLoadCPU       float64 `json:"umbral_load_cpu"`
	ChequesConsecutivos int     `json:"cheques_consecutivos_cpu"`
	UmbralDiscoPercent  float64 `json:"umbral_disco_percent"`
}

// RutaConfig es donde se guarda/lee la configuración de umbrales
const RutaConfig = "system_monitor_logs/config.json"

// ConfigPorDefecto devuelve los umbrales por defecto definidos en el enunciado
func ConfigPorDefecto() ConfigAlertas {
	return ConfigAlertas{
		UmbralRAMPercent:    90.0,
		UmbralLoadCPU:       5.0,
		ChequesConsecutivos: 3,
		UmbralDiscoPercent:  85.0,
	}
}

// LeerConfig lee la configuración desde system_monitor_logs/config.json.
// Si el archivo no existe todavía, devuelve los valores por defecto sin error.
func LeerConfig() (ConfigAlertas, error) {
	datos, err := os.ReadFile(RutaConfig)
	if err != nil {
		if os.IsNotExist(err) {
			return ConfigPorDefecto(), nil
		}
		return ConfigPorDefecto(), err
	}

	config := ConfigPorDefecto()
	if err := json.Unmarshal(datos, &config); err != nil {
		return ConfigPorDefecto(), err
	}

	return config, nil
}

// GuardarConfig escribe la configuración de umbrales en system_monitor_logs/config.json
func GuardarConfig(config ConfigAlertas) error {
	datos, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(RutaConfig, datos, 0644)
}

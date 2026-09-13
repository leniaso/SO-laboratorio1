# SO-laboratorio1

Advanced System Monitor — Práctica #1A del curso de Sistemas Operativos (Universidad de Antioquia).

Herramienta de monitoreo de recursos del sistema con historial, detección de anomalías, alertas configurables y generación de reportes. Implementada en Go en lugar de Bash, usando las interfaces nativas de Linux (`/proc`, `syscall`) y comandos del sistema (`ps`, `ss`, `ip`).

## Estructura del proyecto

```
SO-laboratorio1/
├── go.mod
├── advanced_system_monitor.go   # Task 1 — recolección de datos (main)
├── alert_system.go              # Task 2 — sistema de alertas (main)
├── generate_report.go           # Task 3 — generación de reportes (main)
├── main_monitor.go              # Task 4 — menú/integración (main)
├── monitor/                     # paquete compartido
│   ├── memoria.go                (Task 1) lectura de /proc/meminfo
│   ├── cpu.go                    (Task 1) lectura de /proc/stat
│   ├── red.go                    (Task 1) lectura de /proc/net/dev, ss, ip
│   ├── log.go                    (Task 1) CSV, promedios, detección de fuga, top procesos
│   ├── alertas.go                (Task 2) load average, uso de disco
│   ├── config.go                 (Task 4) umbrales configurables (config.json)
│   └── reportes.go               (Task 3) uptime, picos, promedios históricos, generación de reportes
└── system_monitor_logs/         # se crea en tiempo de ejecución (ignorado por git)
    ├── monitor_log.csv
    ├── procesos_log.csv
    ├── alerts.log
    ├── alert_state.json
    ├── config.json
    ├── daemon.pid
    ├── daemon_output.log
    └── reporte_YYYY-MM-DD.{txt,csv,html}
```

Cada uno de los 4 archivos "main" (`advanced_system_monitor.go`, `alert_system.go`, `generate_report.go`, `main_monitor.go`) es un programa independiente de un solo archivo — por eso se compila pasándole el archivo explícito a `go build`, no `go build .`.

## Requisitos

- Go 1.22+ (el `go.mod` especifica 1.27, pero versiones 1.22+ deberían funcionar)
- Linux (nativo, VM, o WSL2 con Ubuntu/Debian/Alpine)
- Comandos del sistema: `ps`, `ss`, `ip` (en Debian/Ubuntu vienen por defecto; en Alpine hay que instalar `procps` e `iproute2`)

### Instalar dependencias (Ubuntu/Debian/WSL)

```bash
sudo apt update
sudo apt install -y golang-go procps iproute2
```

### Instalar dependencias (Alpine)

```sh
apk update
apk add go procps iproute2 util-linux
```

## Compilación

Desde la raíz del proyecto:

```bash
go build -o monitor_app  advanced_system_monitor.go
go build -o alert_app    alert_system.go
go build -o report_app   generate_report.go
go build -o main_monitor main_monitor.go
```

O todo de una vez:

```bash
for f in advanced_system_monitor:monitor_app alert_system:alert_app generate_report:report_app main_monitor:main_monitor; do
  src="${f%%:*}.go"; bin="${f##*:}"
  go build -o "$bin" "$src"
done
```

Esto genera 4 binarios: `monitor_app`, `alert_app`, `report_app`, `main_monitor`.

## Uso

### 1. `monitor_app` — Recolección de datos (Task 1)

```bash
./monitor_app                 # una ronda: memoria + CPU + red, guarda en CSV
./monitor_app --daemon        # recolección continua cada 5 minutos (bloquea la terminal)
./monitor_app --promedio      # promedio de memoria de la última hora
./monitor_app --fuga          # compara memoria actual vs. promedio de la hora (detección de fuga)
./monitor_app --procesos      # top 5 procesos por CPU, con historial de apariciones
./monitor_app --red           # estado de interfaces de red y conexiones activas
```

Genera `system_monitor_logs/monitor_log.csv` y `system_monitor_logs/procesos_log.csv`.

### 2. `alert_app` — Sistema de alertas (Task 2)

```bash
./alert_app             # una sola verificación
./alert_app --daemon    # verificación continua cada minuto (bloquea la terminal)
```

Umbrales por defecto (configurables, ver sección de configuración más abajo):
- RAM > 90%
- Load average > 5 durante 3 comprobaciones consecutivas
- Disco > 85%
- Cualquier interfaz de red (excepto `lo`) caída

Alertas se imprimen en consola (rojo = crítico, amarillo = advertencia) y se guardan en `system_monitor_logs/alerts.log`. Hay una ventana anti-flood de 5 minutos por tipo de alerta.

### 3. `report_app` — Generación de reportes (Task 3)

```bash
./report_app                     # reporte diario, en texto + CSV + HTML
./report_app --periodo=semanal   # últimos 7 días
./report_app --periodo=48h       # duración personalizada (formato de Go: 48h, 30m, 72h30m...)
./report_app --texto             # solo formato texto
./report_app --csv               # solo formato CSV
./report_app --html              # solo formato HTML
```

Incluye: uptime, load average, pico y promedio de memoria (con fecha), promedio de CPU, top 5 procesos por CPU promedio histórico, y tráfico de red total (RX/TX en MB, errores y drops). Se guarda en `system_monitor_logs/reporte_YYYY-MM-DD.{txt,csv,html}`.

> Necesita que `monitor_app` ya se haya ejecutado al menos una vez (para tener datos en `monitor_log.csv`); si no hay datos en el periodo pedido, el reporte lo indica en vez de fallar.

### 4. `main_monitor` — Menú de integración (Task 4)

**Modo interactivo:**

```bash
./main_monitor
```

Menú:
1. Iniciar daemon de monitoreo (lanza `monitor_app --daemon` en segundo plano, sin bloquear)
2. Detener daemon de monitoreo
3. Ver estadísticas en tiempo real (memoria, CPU, load, disco, red — al instante, sin tocar el CSV)
4. Generar reporte (elige periodo diario/semanal/personalizado y formato)
5. Configurar umbrales de alertas (interactivo, guarda en `config.json`)
6. Ver historial de alertas (últimas 20 líneas de `alerts.log`)
7. Verificar alertas ahora (ejecuta `alert_app` una vez)
0. Salir

**Modo por argumentos (sin menú):**

```bash
./main_monitor --daemon    # inicia el daemon en segundo plano y regresa el control
./main_monitor --report    # genera el reporte diario completo (txt+csv+html)
./main_monitor --alert     # verifica alertas inmediatamente
./main_monitor --config    # editor interactivo de umbrales
```

> `--daemon` y `--alert` requieren que `monitor_app` y `alert_app` ya estén compilados en el mismo directorio (`main_monitor` los ejecuta como procesos externos). Si faltan, `main_monitor` te dice exactamente qué comando correr para compilarlos.

## Configuración de umbrales de alerta

`alert_app` lee `system_monitor_logs/config.json` en cada verificación. Si no existe, usa los valores por defecto del enunciado (RAM 90%, load 5, 3 comprobaciones consecutivas, disco 85%).

Editarlo a mano:
```json
{
  "umbral_ram_percent": 90,
  "umbral_load_cpu": 5,
  "cheques_consecutivos_cpu": 3,
  "umbral_disco_percent": 85
}
```

O de forma interactiva con `./main_monitor --config` (opción 5 del menú).

## Flujo de prueba recomendado

```bash
# 1. Genera algo de historial
./monitor_app
./monitor_app --procesos
sleep 2 && ./monitor_app

# 2. Revisa que el historial se está guardando
cat system_monitor_logs/monitor_log.csv
cat system_monitor_logs/procesos_log.csv

# 3. Verifica alertas (no debería pasar nada si tu sistema está normal)
./alert_app

# 4. Fuerza una alerta para probar el flujo completo
echo '{"umbral_ram_percent": 1, "umbral_load_cpu": 5, "cheques_consecutivos_cpu": 3, "umbral_disco_percent": 85}' > system_monitor_logs/config.json
./alert_app
cat system_monitor_logs/alerts.log
# vuelve a subir el umbral a 90 cuando termines de probar

# 5. Genera un reporte
./report_app
cat system_monitor_logs/reporte_*.txt

# 6. Prueba el daemon vía main_monitor
./main_monitor --daemon
ps -p $(cat system_monitor_logs/daemon.pid)
./main_monitor   # opción 2 para detenerlo
```
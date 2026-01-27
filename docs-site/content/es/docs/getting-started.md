---
title: Comenzar
weight: 2
---

Esta guia te llevara a traves de la instalacion, configuracion y ejecucion de CC-Relay por primera vez.

## Requisitos Previos

- **Go 1.21+** para compilar desde el codigo fuente
- **API keys** de al menos un proveedor soportado (Anthropic o Z.AI)
- **Claude Code** CLI para pruebas (opcional)

## Instalacion

### Usando Go Install

```bash
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest
```

El binario se instalara en `$GOPATH/bin/cc-relay` o `$HOME/go/bin/cc-relay`.

### Compilar desde el Codigo Fuente

```bash
# Clonar el repositorio
git clone https://github.com/omarluq/cc-relay.git
cd cc-relay

# Compilar usando task (recomendado)
task build

# O compilar manualmente
go build -o cc-relay ./cmd/cc-relay

# Ejecutar
./cc-relay --help
```

### Binarios Precompilados

Descarga binarios precompilados desde la [pagina de releases](https://github.com/omarluq/cc-relay/releases).

## Inicio Rapido

### 1. Inicializar Configuracion

CC-Relay puede generar un archivo de configuracion predeterminado:

```bash
cc-relay config init
```

Esto crea un archivo de configuracion en `~/.config/cc-relay/config.yaml` con valores predeterminados razonables.

### 2. Establecer Variables de Entorno

```bash
export ANTHROPIC_API_KEY="tu-api-key-aqui"

# Opcional: Si usas Z.AI
export ZAI_API_KEY="tu-zai-key-aqui"
```

### 3. Ejecutar CC-Relay

```bash
cc-relay serve
```

Deberas ver una salida como:

```
INF starting cc-relay listen=127.0.0.1:8787
INF using primary provider provider=anthropic-pool type=anthropic
```

### 4. Configurar Claude Code

La forma mas facil de configurar Claude Code para usar CC-Relay:

```bash
cc-relay config cc init
```

Esto actualiza automaticamente `~/.claude/settings.json` con la configuracion del proxy.

Alternativamente, establece las variables de entorno manualmente:

```bash
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_AUTH_TOKEN="managed-by-cc-relay"
claude
```

## Verificar que Funciona

### Revisar Estado del Servidor

```bash
cc-relay status
```

Salida:
```
✓ cc-relay is running (127.0.0.1:8787)
```

### Probar el Endpoint de Salud

```bash
curl http://localhost:8787/health
```

Respuesta:
```json
{"status":"ok"}
```

### Listar Modelos Disponibles

```bash
curl http://localhost:8787/v1/models
```

### Probar una Solicitud

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [
      {"role": "user", "content": "Hola!"}
    ]
  }'
```

## Comandos CLI

CC-Relay proporciona varios comandos CLI:

| Comando | Descripcion |
|---------|-------------|
| `cc-relay serve` | Iniciar el servidor proxy |
| `cc-relay status` | Verificar si el servidor esta ejecutandose |
| `cc-relay config init` | Generar archivo de configuracion predeterminado |
| `cc-relay config cc init` | Configurar Claude Code para usar cc-relay |
| `cc-relay config cc remove` | Remover configuracion de cc-relay de Claude Code |
| `cc-relay --version` | Mostrar informacion de version |

### Opciones del Comando Serve

```bash
cc-relay serve [flags]

Flags:
  --config string      Ruta del archivo de configuracion (default: ~/.config/cc-relay/config.yaml)
  --log-level string   Nivel de log (debug, info, warn, error)
  --log-format string  Formato de log (json, text)
  --debug              Habilitar modo debug (logging detallado)
```

## Configuracion Minima

Aqui hay una configuracion minima funcional:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8787"

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"

[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
```
  {{< /tab >}}
{{< /tabs >}}

## Siguientes Pasos

- [Configurar multiples proveedores](/es/docs/configuration/)
- [Entender la arquitectura](/es/docs/architecture/)
- [Referencia de API](/es/docs/api/)

## Solucion de Problemas

### Puerto Ya en Uso

Si el puerto 8787 ya esta en uso, cambia la direccion de escucha en tu configuracion:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8788"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8788"
```
  {{< /tab >}}
{{< /tabs >}}

### Proveedor No Responde

Revisa los logs del servidor para errores de conexion:

```bash
cc-relay serve --log-level debug
```

### Errores de Autenticacion

Si ves errores de "authentication failed":

1. Verifica que tu API key este correctamente establecida en las variables de entorno
2. Revisa que el archivo de configuracion referencie la variable de entorno correcta
3. Asegurate de que la API key sea valida con el proveedor

### Modo Debug

Habilita el modo debug para registro detallado de solicitudes/respuestas:

```bash
cc-relay serve --debug
```

Esto habilita:
- Nivel de log debug
- Registro del cuerpo de solicitudes (campos sensibles redactados)
- Registro de headers de respuesta
- Metricas de conexion TLS

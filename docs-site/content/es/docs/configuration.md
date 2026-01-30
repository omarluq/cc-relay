---
title: Configuracion
weight: 3
---

CC-Relay se configura mediante archivos YAML o TOML. Esta guia cubre todas las opciones de configuracion.

## Ubicacion del Archivo de Configuracion

Ubicaciones predeterminadas (revisadas en orden):

1. `./config.yaml` o `./config.toml` (directorio actual)
2. `~/.config/cc-relay/config.yaml` o `~/.config/cc-relay/config.toml`
3. Ruta especificada via flag `--config`

El formato se detecta automaticamente por la extension del archivo (`.yaml`, `.yml` o `.toml`).

Genera una configuracion predeterminada con:

```bash
cc-relay config init
```

## Expansion de Variables de Entorno

CC-Relay soporta expansion de variables de entorno usando la sintaxis `${VAR_NAME}` en ambos formatos YAML y TOML:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # Expandida al cargar
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"  # Expandida al cargar
```
  {{< /tab >}}
{{< /tabs >}}

## Referencia Completa de Configuracion

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# ==========================================================================
# Configuracion del Servidor
# ==========================================================================
server:
  # Direccion de escucha
  listen: "127.0.0.1:8787"

  # Timeout de solicitud en milisegundos (default: 600000 = 10 minutos)
  timeout_ms: 600000

  # Solicitudes concurrentes maximas (0 = ilimitado)
  max_concurrent: 0

  # Habilitar HTTP/2 para mejor rendimiento
  enable_http2: true

  # Configuracion de autenticacion
  auth:
    # Requerir API key especifica para acceso al proxy
    api_key: "${PROXY_API_KEY}"

    # Permitir Bearer tokens de suscripcion de Claude Code
    allow_subscription: true

    # Bearer token especifico para validar (opcional)
    bearer_secret: "${BEARER_SECRET}"

# ==========================================================================
# Configuraciones de Proveedores
# ==========================================================================
providers:
  # API Directa de Anthropic
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Opcional, usa valor predeterminado

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60       # Solicitudes por minuto
        tpm_limit: 100000   # Tokens por minuto

    # Opcional: Especificar modelos disponibles
    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"

  # Z.AI / Zhipu GLM
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"

    keys:
      - key: "${ZAI_API_KEY}"

    # Mapear nombres de modelos Claude a modelos Z.AI
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    # Opcional: Especificar modelos disponibles
    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"

# ==========================================================================
# Configuracion de Logging
# ==========================================================================
logging:
  # Nivel de log: debug, info, warn, error
  level: "info"

  # Formato de log: json, text
  format: "text"

  # Habilitar salida con colores (para formato text)
  pretty: true

  # Opciones granulares de debug
  debug_options:
    log_request_body: false
    log_response_headers: false
    log_tls_metrics: false
    max_body_log_size: 1000

# ==========================================================================
# Configuracion de Cache
# ==========================================================================
cache:
  # Modo de cache: single, ha, disabled
  mode: single

  # Configuracion de modo unico (Ristretto)
  ristretto:
    num_counters: 1000000  # 10x elementos maximos esperados
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Tamano del buffer de admision

  # Configuracion de modo HA (Olric)
  olric:
    embedded: true                 # Ejecutar nodo Olric embebido
    bind_addr: "0.0.0.0:3320"      # Puerto de cliente Olric
    dmap_name: "cc-relay"          # Nombre del mapa distribuido
    environment: lan               # local, lan, o wan
    peers:                         # Direcciones memberlist (bind_addr + 2)
      - "other-node:3322"
    replica_count: 2               # Copias por clave
    read_quorum: 1                 # Min. lecturas para exito
    write_quorum: 1                # Min. escrituras para exito
    member_count_quorum: 2         # Min. miembros del cluster
    leave_timeout: 5s              # Duracion del mensaje de salida

# ==========================================================================
# Configuracion de Routing
# ==========================================================================
routing:
  # Estrategia: round_robin, weighted_round_robin, shuffle, failover (predeterminado)
  strategy: failover

  # Timeout para intentos de failover en milisegundos (predeterminado: 5000)
  failover_timeout: 5000

  # Habilitar headers de debug (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false
```
  {{< /tab >}}
  {{< tab >}}
```toml
# ==========================================================================
# Server Configuration
# ==========================================================================
[server]
# Address to listen on
listen = "127.0.0.1:8787"

# Request timeout in milliseconds (default: 600000 = 10 minutes)
timeout_ms = 600000

# Maximum concurrent requests (0 = unlimited)
max_concurrent = 0

# Enable HTTP/2 for better performance
enable_http2 = true

# Authentication configuration
[server.auth]
# Require specific API key for proxy access
api_key = "${PROXY_API_KEY}"

# Allow Claude Code subscription Bearer tokens
allow_subscription = true

# Specific Bearer token to validate (optional)
bearer_secret = "${BEARER_SECRET}"

# ==========================================================================
# Provider Configurations
# ==========================================================================

# Anthropic Direct API
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true
base_url = "https://api.anthropic.com"  # Optional, uses default

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60       # Requests per minute
tpm_limit = 100000   # Tokens per minute

# Optional: Specify available models
models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]

# Z.AI / Zhipu GLM
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"

[[providers.keys]]
key = "${ZAI_API_KEY}"

# Map Claude model names to Z.AI models
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"

# Optional: Specify available models
models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]

# ==========================================================================
# Logging Configuration
# ==========================================================================
[logging]
# Log level: debug, info, warn, error
level = "info"

# Log format: json, text
format = "text"

# Enable colored output (for text format)
pretty = true

# Granular debug options
[logging.debug_options]
log_request_body = false
log_response_headers = false
log_tls_metrics = false
max_body_log_size = 1000

# ==========================================================================
# Cache Configuration
# ==========================================================================
[cache]
# Cache mode: single, ha, disabled
mode = "single"

# Single mode (Ristretto) configuration
[cache.ristretto]
num_counters = 1000000  # 10x expected max items
max_cost = 104857600    # 100 MB
buffer_items = 64       # Admission buffer size

# HA mode (Olric) configuration
[cache.olric]
embedded = true                 # Run embedded Olric node
bind_addr = "0.0.0.0:3320"      # Olric client port
dmap_name = "cc-relay"          # Distributed map name
environment = "lan"             # local, lan, or wan
peers = ["other-node:3322"]     # Memberlist addresses (bind_addr + 2)
replica_count = 2               # Copies per key
read_quorum = 1                 # Min reads for success
write_quorum = 1                # Min writes for success
member_count_quorum = 2         # Min cluster members
leave_timeout = "5s"            # Leave broadcast duration

# ==========================================================================
# Routing Configuration
# ==========================================================================
[routing]
# Strategy: round_robin, weighted_round_robin, shuffle, failover (default)
strategy = "failover"

# Timeout for failover attempts in milliseconds (default: 5000)
failover_timeout = 5000

# Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
debug = false
```
  {{< /tab >}}
{{< /tabs >}}

## Configuracion del Servidor

### Direccion de Escucha

El campo `listen` especifica donde el proxy escucha solicitudes entrantes:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8787"  # Solo local (recomendado)
  # listen: "0.0.0.0:8787"  # Todas las interfaces (usar con precaucion)
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"  # Solo local (recomendado)
# listen = "0.0.0.0:8787"  # Todas las interfaces (usar con precaucion)
```
  {{< /tab >}}
{{< /tabs >}}

### Autenticacion

CC-Relay soporta multiples metodos de autenticacion:

#### Autenticacion con API Key

Requerir que los clientes proporcionen una API key especifica:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
api_key = "${PROXY_API_KEY}"
```
  {{< /tab >}}
{{< /tabs >}}

Los clientes deben incluir el header: `x-api-key: <tu-proxy-key>`

#### Passthrough de Suscripcion Claude Code

Permitir que usuarios con suscripcion de Claude Code se conecten:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    allow_subscription: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
allow_subscription = true
```
  {{< /tab >}}
{{< /tabs >}}

Esto acepta tokens `Authorization: Bearer` de Claude Code.

#### Autenticacion Combinada

Permitir tanto API key como autenticacion de suscripcion:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
api_key = "${PROXY_API_KEY}"
allow_subscription = true
```
  {{< /tab >}}
{{< /tabs >}}

#### Sin Autenticacion

Para deshabilitar autenticacion (no recomendado para produccion):

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth: {}
  # O simplemente omitir la seccion auth
```
  {{< /tab >}}
  {{< tab >}}
```toml
# Omitir la seccion [server.auth] completamente
# o usar una seccion vacia:
[server]
# (sin bloque auth)
```
  {{< /tab >}}
{{< /tabs >}}

### Soporte HTTP/2

Habilitar HTTP/2 para mejor rendimiento con solicitudes concurrentes:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  enable_http2: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
enable_http2 = true
```
  {{< /tab >}}
{{< /tabs >}}

## Configuracion de Proveedores

### Tipos de Proveedores

CC-Relay actualmente soporta dos tipos de proveedores:

| Tipo | Descripcion | URL Base Predeterminada |
|------|-------------|-------------------------|
| `anthropic` | API Directa de Anthropic | `https://api.anthropic.com` |
| `zai` | Z.AI / Zhipu GLM | `https://api.z.ai/api/anthropic` |

### Proveedor Anthropic

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Opcional

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60
        tpm_limit: 100000

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true
base_url = "https://api.anthropic.com"  # Opcional

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60
tpm_limit = 100000

models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]
```
  {{< /tab >}}
{{< /tabs >}}

### Proveedor Z.AI

Z.AI ofrece APIs compatibles con Anthropic con modelos GLM a menor costo:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"

    keys:
      - key: "${ZAI_API_KEY}"

    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"

[[providers.keys]]
key = "${ZAI_API_KEY}"

[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"

models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]
```
  {{< /tab >}}
{{< /tabs >}}

### Multiples API Keys

Agrupar multiples API keys para mayor rendimiento:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true

    keys:
      - key: "${ANTHROPIC_API_KEY_1}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_2}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_3}"
        rpm_limit: 60
        tpm_limit: 100000
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_1}"
rpm_limit = 60
tpm_limit = 100000

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_2}"
rpm_limit = 60
tpm_limit = 100000

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_3}"
rpm_limit = 60
tpm_limit = 100000
```
  {{< /tab >}}
{{< /tabs >}}

### URL Base Personalizada

Sobreescribir el endpoint de API predeterminado:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic-custom"
type = "anthropic"
base_url = "https://custom-endpoint.example.com"
```
  {{< /tab >}}
{{< /tabs >}}

## Configuracion de Logging

### Niveles de Log

| Nivel | Descripcion |
|-------|-------------|
| `debug` | Salida detallada para desarrollo |
| `info` | Mensajes de operacion normal |
| `warn` | Mensajes de advertencia |
| `error` | Solo mensajes de error |

### Formato de Log

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
logging:
  format: "text"   # Legible por humanos (predeterminado)
  # format: "json" # Legible por maquinas, para agregacion de logs
```
  {{< /tab >}}
  {{< tab >}}
```toml
[logging]
format = "text"   # Legible por humanos (predeterminado)
# format = "json" # Legible por maquinas, para agregacion de logs
```
  {{< /tab >}}
{{< /tabs >}}

### Opciones de Debug

Control detallado sobre el logging de debug:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # Registrar cuerpos de solicitud (redactados)
    log_response_headers: true  # Registrar headers de respuesta
    log_tls_metrics: true       # Registrar info de conexion TLS
    max_body_log_size: 1000     # Bytes maximos a registrar de cuerpos
```
  {{< /tab >}}
  {{< tab >}}
```toml
[logging]
level = "debug"

[logging.debug_options]
log_request_body = true      # Registrar cuerpos de solicitud (redactados)
log_response_headers = true  # Registrar headers de respuesta
log_tls_metrics = true       # Registrar info de conexion TLS
max_body_log_size = 1000     # Bytes maximos a registrar de cuerpos
```
  {{< /tab >}}
{{< /tabs >}}

## Configuracion de Cache

CC-Relay proporciona una capa de cache unificada con multiples opciones de backend para diferentes escenarios de implementacion.

### Modos de Cache

| Modo | Backend | Descripcion |
|------|---------|-------------|
| `single` | [Ristretto](https://github.com/dgraph-io/ristretto) | Cache local en memoria de alto rendimiento (predeterminado) |
| `ha` | [Olric](https://github.com/buraksezer/olric) | Cache distribuido para implementaciones de alta disponibilidad |
| `disabled` | Noop | Modo de paso sin almacenamiento en cache |

### Modo Unico (Ristretto)

Ristretto es un cache en memoria concurrente de alto rendimiento. Este es el modo predeterminado para implementaciones de instancia unica.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x elementos maximos esperados
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Tamano del buffer de admision
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "single"

[cache.ristretto]
num_counters = 1000000  # 10x elementos maximos esperados
max_cost = 104857600    # 100 MB
buffer_items = 64       # Tamano del buffer de admision
```
  {{< /tab >}}
{{< /tabs >}}

| Campo | Tipo | Predeterminado | Descripcion |
|-------|------|----------------|-------------|
| `num_counters` | int64 | 1,000,000 | Numero de contadores de acceso de 4 bits. Recomendado: 10x elementos maximos esperados. |
| `max_cost` | int64 | 104,857,600 (100 MB) | Memoria maxima en bytes que el cache puede contener. |
| `buffer_items` | int64 | 64 | Numero de claves por buffer de Get. Controla el tamano del buffer de admision. |

### Modo HA (Olric) - Embebido

Para implementaciones multi-instancia que requieren estado de cache compartido, use el modo Olric embebido donde cada instancia de cc-relay ejecuta un nodo Olric.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "other-node:3322"  # Puerto memberlist = bind_addr + 2
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "ha"

[cache.olric]
embedded = true
bind_addr = "0.0.0.0:3320"
dmap_name = "cc-relay"
environment = "lan"
peers = ["other-node:3322"]  # Puerto memberlist = bind_addr + 2
replica_count = 2
read_quorum = 1
write_quorum = 1
member_count_quorum = 2
leave_timeout = "5s"
```
  {{< /tab >}}
{{< /tabs >}}

| Campo | Tipo | Predeterminado | Descripcion |
|-------|------|----------------|-------------|
| `embedded` | bool | false | Ejecutar nodo Olric embebido (true) vs. conectar a cluster externo (false). |
| `bind_addr` | string | requerido | Direccion para conexiones de cliente Olric (ej. "0.0.0.0:3320"). |
| `dmap_name` | string | "cc-relay" | Nombre del mapa distribuido. Todos los nodos deben usar el mismo nombre. |
| `environment` | string | "local" | Preset de memberlist: "local", "lan", o "wan". |
| `peers` | []string | - | Direcciones memberlist para descubrimiento de peers. Usa puerto bind_addr + 2. |
| `replica_count` | int | 1 | Numero de copias por clave. 1 = sin replicacion. |
| `read_quorum` | int | 1 | Lecturas exitosas minimas para respuesta. |
| `write_quorum` | int | 1 | Escrituras exitosas minimas para respuesta. |
| `member_count_quorum` | int32 | 1 | Miembros minimos del cluster requeridos para operar. |
| `leave_timeout` | duration | 5s | Tiempo para transmitir mensaje de salida antes de apagar. |

**Importante:** Olric usa dos puertos - el puerto `bind_addr` para conexiones de cliente y `bind_addr + 2` para gossip de memberlist. Asegurese de que ambos puertos esten abiertos en su firewall.

### Modo HA (Olric) - Modo Cliente

Conecte a un cluster Olric externo en lugar de ejecutar nodos embebidos:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: ha
  olric:
    embedded: false
    addresses:
      - "olric-node-1:3320"
      - "olric-node-2:3320"
    dmap_name: "cc-relay"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "ha"

[cache.olric]
embedded = false
addresses = ["olric-node-1:3320", "olric-node-2:3320"]
dmap_name = "cc-relay"
```
  {{< /tab >}}
{{< /tabs >}}

| Campo | Tipo | Descripcion |
|-------|------|-------------|
| `embedded` | bool | Establecer en `false` para modo cliente. |
| `addresses` | []string | Direcciones del cluster Olric externo. |
| `dmap_name` | string | Nombre del mapa distribuido (debe coincidir con la configuracion del cluster). |

### Modo Deshabilitado

Deshabilitar el cache completamente para depuracion o cuando el cache se maneja en otro lugar:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: disabled
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "disabled"
```
  {{< /tab >}}
{{< /tabs >}}

Para documentacion completa de cache incluyendo convenciones de claves de cache, estrategias de invalidacion de cache, guias de clustering HA y solucion de problemas, vea la [documentacion del Sistema de Cache](/es/docs/caching/).

## Configuracion de Routing

CC-Relay soporta multiples estrategias de routing para distribuir solicitudes entre proveedores.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# ==========================================================================
# Configuracion de Routing
# ==========================================================================
routing:
  # Estrategia: round_robin, weighted_round_robin, shuffle, failover (predeterminado)
  strategy: failover

  # Timeout para intentos de failover en milisegundos (predeterminado: 5000)
  failover_timeout: 5000

  # Habilitar headers de debug (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false
```
  {{< /tab >}}
  {{< tab >}}
```toml
# ==========================================================================
# Configuracion de Routing
# ==========================================================================
[routing]
# Estrategia: round_robin, weighted_round_robin, shuffle, failover (predeterminado)
strategy = "failover"

# Timeout para intentos de failover en milisegundos (predeterminado: 5000)
failover_timeout = 5000

# Habilitar headers de debug (X-CC-Relay-Strategy, X-CC-Relay-Provider)
debug = false
```
  {{< /tab >}}
{{< /tabs >}}

### Estrategias de Routing

| Estrategia | Descripcion |
|------------|-------------|
| `failover` | Intentar proveedores en orden de prioridad, fallback en caso de fallo (predeterminado) |
| `round_robin` | Rotacion secuencial a traves de proveedores |
| `weighted_round_robin` | Distribucion proporcional por peso |
| `shuffle` | Distribucion aleatoria justa |

### Peso y Prioridad de Proveedor

El peso y la prioridad se configuran en la primera clave del proveedor:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # Para weighted-round-robin (mayor = mas trafico)
        priority: 2    # Para failover (mayor = se intenta primero)
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
weight = 3      # Para weighted-round-robin (mayor = mas trafico)
priority = 2    # Para failover (mayor = se intenta primero)
```
  {{< /tab >}}
{{< /tabs >}}

Para configuracion detallada de routing incluyendo explicaciones de estrategias, cabeceras de depuracion y disparadores de failover, vea la [documentacion de Routing](/es/docs/routing/).

## Configuraciones de Ejemplo

### Minima con Un Solo Proveedor

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

### Configuracion Multi-Proveedor

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8787"
  auth:
    allow_subscription: true

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"

logging:
  level: "info"
  format: "text"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"

[server.auth]
allow_subscription = true

[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"

[[providers]]
name = "zai"
type = "zai"
enabled = true

[[providers.keys]]
key = "${ZAI_API_KEY}"

[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"

[logging]
level = "info"
format = "text"
```
  {{< /tab >}}
{{< /tabs >}}

### Desarrollo con Logging de Debug

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

logging:
  level: "debug"
  format: "text"
  pretty: true
  debug_options:
    log_request_body: true
    log_response_headers: true
    log_tls_metrics: true
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

[logging]
level = "debug"
format = "text"
pretty = true

[logging.debug_options]
log_request_body = true
log_response_headers = true
log_tls_metrics = true
```
  {{< /tab >}}
{{< /tabs >}}

## Validar Configuracion

Valida tu archivo de configuracion:

```bash
cc-relay config validate
```

**Consejo**: Siempre valida los cambios de configuracion antes de desplegar. La recarga en caliente rechazara configuraciones invalidas, pero la validacion detecta errores antes de que lleguen a produccion.

## Recarga en Caliente

CC-Relay detecta y aplica automaticamente los cambios de configuracion sin requerir un reinicio. Esto permite actualizaciones de configuracion sin tiempo de inactividad.

### Como Funciona

CC-Relay usa [fsnotify](https://github.com/fsnotify/fsnotify) para monitorear el archivo de configuracion:

1. **Monitoreo de archivos**: Se monitorea el directorio padre para detectar correctamente las escrituras atomicas (archivo temporal + renombrado usado por la mayoria de editores)
2. **Antirrebote**: Multiples eventos de archivo rapidos se agrupan con un retardo de 100ms para manejar el comportamiento de guardado de editores
3. **Intercambio atomico**: La nueva configuracion se carga e intercambia atomicamente usando `sync/atomic.Pointer` de Go
4. **Preservacion de solicitudes en curso**: Las solicitudes en progreso continuan con la configuracion anterior; las nuevas solicitudes usan la configuracion actualizada

### Eventos que Disparan la Recarga

| Evento | Dispara Recarga |
|--------|-----------------|
| Escritura de archivo | Si |
| Creacion de archivo (renombrado atomico) | Si |
| Chmod de archivo | No (ignorado) |
| Otro archivo en el directorio | No (ignorado) |

### Registro

Cuando ocurre una recarga en caliente, veras mensajes de registro:

```
INF config file reloaded path=/path/to/config.yaml
INF config hot-reloaded successfully
```

Si la nueva configuracion es invalida:

```
ERR failed to reload config path=/path/to/config.yaml error="validation error"
```

Las configuraciones invalidas se rechazan y el proxy continua con la configuracion valida anterior.

### Limitaciones

- **Direccion de escucha**: Cambiar `server.listen` requiere reinicio
- **Direccion gRPC**: Cambiar `grpc.listen` requiere reinicio

Opciones de configuracion que pueden recargarse en caliente:
- Nivel y formato de registro
- Estrategia de enrutamiento, timeout de failover, pesos y prioridades
- Activacion de providers, base URL y mapeo de modelos
- Estrategia de keypool, pesos de claves y limites por clave
- Maximo de solicitudes concurrentes y tamano maximo del body
- Intervalos de salud y umbrales del circuit breaker

### Garantias de hot-reload

- Las nuevas solicitudes usan la configuracion mas reciente tras la recarga.
- Las solicitudes en curso continúan con la configuracion anterior.
- La recarga se aplica de forma atomica a routing/providers/keypool.
- Las configuraciones invalidas se rechazan y la anterior permanece activa.
## Siguientes Pasos

- [Estrategias de routing](/es/docs/routing/) - Seleccion de proveedor y failover
- [Entender la arquitectura](/es/docs/architecture/)
- [Referencia de API](/es/docs/api/)

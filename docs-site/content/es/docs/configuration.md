---
title: Configuracion
weight: 3
---

CC-Relay se configura mediante archivos YAML. Esta guia cubre todas las opciones de configuracion.

## Ubicacion del Archivo de Configuracion

Ubicaciones predeterminadas (revisadas en orden):

1. `./config.yaml` (directorio actual)
2. `~/.config/cc-relay/config.yaml`
3. Ruta especificada via flag `--config`

Genera una configuracion predeterminada con:

```bash
cc-relay config init
```

## Expansion de Variables de Entorno

CC-Relay soporta expansion de variables de entorno usando la sintaxis `${VAR_NAME}`:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # Expandida al cargar
```

## Referencia Completa de Configuracion

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

## Configuracion del Servidor

### Direccion de Escucha

El campo `listen` especifica donde el proxy escucha solicitudes entrantes:

```yaml
server:
  listen: "127.0.0.1:8787"  # Solo local (recomendado)
  # listen: "0.0.0.0:8787"  # Todas las interfaces (usar con precaucion)
```

### Autenticacion

CC-Relay soporta multiples metodos de autenticacion:

#### Autenticacion con API Key

Requerir que los clientes proporcionen una API key especifica:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```

Los clientes deben incluir el header: `x-api-key: <tu-proxy-key>`

#### Passthrough de Suscripcion Claude Code

Permitir que usuarios con suscripcion de Claude Code se conecten:

```yaml
server:
  auth:
    allow_subscription: true
```

Esto acepta tokens `Authorization: Bearer` de Claude Code.

#### Autenticacion Combinada

Permitir tanto API key como autenticacion de suscripcion:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```

#### Sin Autenticacion

Para deshabilitar autenticacion (no recomendado para produccion):

```yaml
server:
  auth: {}
  # O simplemente omitir la seccion auth
```

### Soporte HTTP/2

Habilitar HTTP/2 para mejor rendimiento con solicitudes concurrentes:

```yaml
server:
  enable_http2: true
```

## Configuracion de Proveedores

### Tipos de Proveedores

CC-Relay actualmente soporta dos tipos de proveedores:

| Tipo | Descripcion | URL Base Predeterminada |
|------|-------------|-------------------------|
| `anthropic` | API Directa de Anthropic | `https://api.anthropic.com` |
| `zai` | Z.AI / Zhipu GLM | `https://api.z.ai/api/anthropic` |

### Proveedor Anthropic

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

### Proveedor Z.AI

Z.AI ofrece APIs compatibles con Anthropic con modelos GLM a menor costo:

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

### Multiples API Keys

Agrupar multiples API keys para mayor rendimiento:

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

### URL Base Personalizada

Sobreescribir el endpoint de API predeterminado:

```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```

## Configuracion de Logging

### Niveles de Log

| Nivel | Descripcion |
|-------|-------------|
| `debug` | Salida detallada para desarrollo |
| `info` | Mensajes de operacion normal |
| `warn` | Mensajes de advertencia |
| `error` | Solo mensajes de error |

### Formato de Log

```yaml
logging:
  format: "text"   # Legible por humanos (predeterminado)
  # format: "json" # Legible por maquinas, para agregacion de logs
```

### Opciones de Debug

Control detallado sobre el logging de debug:

```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # Registrar cuerpos de solicitud (redactados)
    log_response_headers: true  # Registrar headers de respuesta
    log_tls_metrics: true       # Registrar info de conexion TLS
    max_body_log_size: 1000     # Bytes maximos a registrar de cuerpos
```

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

```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x elementos maximos esperados
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Tamano del buffer de admision
```

| Campo | Tipo | Predeterminado | Descripcion |
|-------|------|----------------|-------------|
| `num_counters` | int64 | 1,000,000 | Numero de contadores de acceso de 4 bits. Recomendado: 10x elementos maximos esperados. |
| `max_cost` | int64 | 104,857,600 (100 MB) | Memoria maxima en bytes que el cache puede contener. |
| `buffer_items` | int64 | 64 | Numero de claves por buffer de Get. Controla el tamano del buffer de admision. |

### Modo HA (Olric) - Embebido

Para implementaciones multi-instancia que requieren estado de cache compartido, use el modo Olric embebido donde cada instancia de cc-relay ejecuta un nodo Olric.

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

| Campo | Tipo | Descripcion |
|-------|------|-------------|
| `embedded` | bool | Establecer en `false` para modo cliente. |
| `addresses` | []string | Direcciones del cluster Olric externo. |
| `dmap_name` | string | Nombre del mapa distribuido (debe coincidir con la configuracion del cluster). |

### Modo Deshabilitado

Deshabilitar el cache completamente para depuracion o cuando el cache se maneja en otro lugar:

```yaml
cache:
  mode: disabled
```

Para documentacion completa de cache incluyendo convenciones de claves de cache, estrategias de invalidacion de cache, guias de clustering HA y solucion de problemas, vea la [documentacion del Sistema de Cache](/es/docs/caching/).

## Configuracion de Routing

CC-Relay soporta multiples estrategias de routing para distribuir solicitudes entre proveedores.

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

### Estrategias de Routing

| Estrategia | Descripcion |
|------------|-------------|
| `failover` | Intentar proveedores en orden de prioridad, fallback en caso de fallo (predeterminado) |
| `round_robin` | Rotacion secuencial a traves de proveedores |
| `weighted_round_robin` | Distribucion proporcional por peso |
| `shuffle` | Distribucion aleatoria justa |

### Peso y Prioridad de Proveedor

El peso y la prioridad se configuran en la primera clave del proveedor:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # Para weighted-round-robin (mayor = mas trafico)
        priority: 2    # Para failover (mayor = se intenta primero)
```

Para configuracion detallada de routing incluyendo explicaciones de estrategias, cabeceras de depuracion y disparadores de failover, vea la [documentacion de Routing](/es/docs/routing/).

## Configuraciones de Ejemplo

### Minima con Un Solo Proveedor

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

### Configuracion Multi-Proveedor

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

### Desarrollo con Logging de Debug

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

## Validar Configuracion

Valida tu archivo de configuracion:

```bash
cc-relay config validate
```

## Recarga en Caliente

Los cambios de configuracion requieren reiniciar el servidor. La recarga en caliente esta planeada para una version futura.

## Siguientes Pasos

- [Estrategias de routing](/es/docs/routing/) - Seleccion de proveedor y failover
- [Entender la arquitectura](/es/docs/architecture/)
- [Referencia de API](/es/docs/api/)

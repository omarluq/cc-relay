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

- [Entender la arquitectura](/es/docs/architecture/)
- [Referencia de API](/es/docs/api/)

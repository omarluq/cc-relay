---
title: Routing
weight: 4
---

CC-Relay soporta multiples estrategias de routing para distribuir solicitudes entre proveedores. Esta pagina explica cada estrategia y como configurarlas.

## Descripcion General

El routing determina como cc-relay elige que proveedor maneja cada solicitud. La estrategia correcta depende de sus prioridades: disponibilidad, costo, latencia o distribucion de carga.

| Estrategia | Valor de Configuracion | Descripcion | Caso de Uso |
|------------|------------------------|-------------|-------------|
| Round-Robin | `round_robin` | Rotacion secuencial a traves de proveedores | Distribucion uniforme |
| Weighted Round-Robin | `weighted_round_robin` | Distribucion proporcional por peso | Distribucion basada en capacidad |
| Shuffle | `shuffle` | Aleatorio justo ("repartir cartas") | Balanceo de carga aleatorio |
| Failover | `failover` (predeterminado) | Basado en prioridad con reintento automatico | Alta disponibilidad |
| Model-Based | `model_based` | Routing por prefijo de nombre de modelo | Despliegues multi-modelo |

## Configuracion

Configure el routing en su `config.yaml`:

```yaml
routing:
  # Estrategia: round_robin, weighted_round_robin, shuffle, failover (predeterminado), model_based
  strategy: failover

  # Timeout para intentos de failover en milisegundos (predeterminado: 5000)
  failover_timeout: 5000

  # Habilitar headers de debug (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false

  # Configuracion de routing basado en modelo (solo usado cuando strategy: model_based)
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
  default_provider: anthropic
```

**Predeterminado:** Si `strategy` no se especifica, cc-relay usa `failover` como la opcion mas segura.

## Estrategias

### Round-Robin

Distribucion secuencial usando un contador atomico. Cada proveedor recibe una solicitud antes de que cualquier proveedor reciba una segunda.

```yaml
routing:
  strategy: round_robin
```

**Como funciona:**

1. Solicitud 1 → Proveedor A
2. Solicitud 2 → Proveedor B
3. Solicitud 3 → Proveedor C
4. Solicitud 4 → Proveedor A (el ciclo se repite)

**Mejor para:** Distribucion uniforme entre proveedores con capacidad similar.

### Weighted Round-Robin

Distribuye solicitudes proporcionalmente basandose en los pesos de los proveedores. Usa el algoritmo Nginx smooth weighted round-robin para distribucion uniforme.

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3  # Recibe 3x mas solicitudes

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        weight: 1  # Recibe 1x solicitudes
```

**Como funciona:**

Con pesos 3:1, de cada 4 solicitudes:
- 3 solicitudes → anthropic
- 1 solicitud → zai

**Peso predeterminado:** 1 (si no se especifica)

**Mejor para:** Distribuir carga basada en capacidad del proveedor, limites de tasa o asignacion de costos.

### Shuffle

Distribucion aleatoria justa usando el patron Fisher-Yates de "repartir cartas". Todos reciben una carta antes de que alguien reciba una segunda.

```yaml
routing:
  strategy: shuffle
```

**Como funciona:**

1. Todos los proveedores comienzan en un "mazo"
2. Se selecciona un proveedor aleatorio y se remueve del mazo
3. Cuando el mazo esta vacio, se barajan todos los proveedores
4. Garantiza distribucion justa a lo largo del tiempo

**Mejor para:** Balanceo de carga aleatorio mientras se asegura equidad.

### Failover

Intenta proveedores en orden de prioridad. En caso de fallo, lanza carreras paralelas con los proveedores restantes para obtener la respuesta exitosa mas rapida. Esta es la **estrategia predeterminada**.

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # Se intenta primero (mayor = mayor prioridad)

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Respaldo
```

**Como funciona:**

1. Intenta primero el proveedor de mayor prioridad
2. Si falla (ver [Disparadores de Failover](#disparadores-de-failover)), lanza solicitudes paralelas a todos los proveedores restantes
3. Retorna la primera respuesta exitosa, cancela las otras
4. Respeta `failover_timeout` para el tiempo total de la operacion

**Prioridad predeterminada:** 1 (si no se especifica)

**Mejor para:** Alta disponibilidad con fallback automatico.

### Model-Based

Enruta solicitudes a proveedores basandose en el nombre del modelo en la solicitud. Usa coincidencia de prefijo mas largo para especificidad.

```yaml
routing:
  strategy: model_based
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
    llama: ollama
  default_provider: anthropic

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
  - name: "ollama"
    type: "ollama"
    base_url: "http://localhost:11434"
```

**Como funciona:**

1. Extrae el parametro `model` de la solicitud
2. Intenta encontrar la coincidencia de prefijo mas largo en `model_mapping`
3. Enruta al proveedor correspondiente
4. Recurre a `default_provider` si no se encuentra coincidencia
5. Devuelve un error si no hay coincidencia ni predeterminado

**Ejemplos de coincidencia de prefijo:**

| Modelo Solicitado | Entradas de Mapeo | Entrada Seleccionada | Proveedor |
|-------------------|-------------------|---------------------|-----------|
| `claude-opus-4` | `claude-opus`, `claude` | `claude-opus` | anthropic |
| `claude-sonnet-3.5` | `claude-sonnet`, `claude` | `claude-sonnet` | anthropic |
| `glm-4-plus` | `glm-4`, `glm` | `glm-4` | zai |
| `qwen-72b` | `qwen`, `claude` | `qwen` | ollama |
| `llama-3.2` | `llama`, `claude` | `llama` | ollama |
| `gpt-4` | `claude`, `llama` | (sin coincidencia) | default_provider |

**Mejor para:** Despliegues multi-modelo donde diferentes modelos necesitan enrutarse a diferentes proveedores.

## Cabeceras de Depuracion

Cuando `routing.debug: true`, cc-relay agrega cabeceras de diagnostico a las respuestas:

| Cabecera | Valor | Descripcion |
|----------|-------|-------------|
| `X-CC-Relay-Strategy` | Nombre de estrategia | Que estrategia de routing se uso |
| `X-CC-Relay-Provider` | Nombre del proveedor | Que proveedor manejo la solicitud |

**Ejemplo de cabeceras de respuesta:**

```
X-CC-Relay-Strategy: failover
X-CC-Relay-Provider: anthropic
```

**Advertencia de Seguridad:** Las cabeceras de depuracion exponen decisiones internas de routing. Use solo en desarrollo o entornos confiables. Nunca habilite en produccion con clientes no confiables.

## Disparadores de Failover

La estrategia de failover dispara reintento en condiciones de error especificas:

| Disparador | Condiciones | Descripcion |
|------------|-------------|-------------|
| Codigo de Estado | `429`, `500`, `502`, `503`, `504` | Limite de tasa o errores de servidor |
| Timeout | `context.DeadlineExceeded` | Timeout de solicitud excedido |
| Conexion | `net.Error` | Errores de red, fallos DNS, conexion rechazada |

**Importante:** Los errores de cliente (4xx excepto 429) **no** disparan failover. Estos indican problemas con la solicitud misma, no con el proveedor.

### Codigos de Estado Explicados

| Codigo | Significado | Failover? |
|--------|-------------|-----------|
| `429` | Limite de Tasa | Si - intentar otro proveedor |
| `500` | Error Interno del Servidor | Si - problema del servidor |
| `502` | Bad Gateway | Si - problema upstream |
| `503` | Servicio No Disponible | Si - temporalmente caido |
| `504` | Gateway Timeout | Si - timeout upstream |
| `400` | Solicitud Incorrecta | No - corregir la solicitud |
| `401` | No Autorizado | No - corregir autenticacion |
| `403` | Prohibido | No - problema de permisos |

## Ejemplos

### Failover Simple (Recomendado para la Mayoria de Usuarios)

Use la estrategia predeterminada con proveedores priorizados:

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1
```

### Balanceo de Carga con Pesos

Distribuir carga basada en capacidad del proveedor:

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "primary"
    type: "anthropic"
    keys:
      - key: "${PRIMARY_KEY}"
        weight: 3  # 75% del trafico

  - name: "secondary"
    type: "anthropic"
    keys:
      - key: "${SECONDARY_KEY}"
        weight: 1  # 25% del trafico
```

### Desarrollo con Cabeceras de Depuracion

Habilitar cabeceras de depuracion para solucion de problemas:

```yaml
routing:
  strategy: round_robin
  debug: true

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
```

### Alta Disponibilidad con Failover Rapido

Minimizar latencia de failover:

```yaml
routing:
  strategy: failover
  failover_timeout: 3000  # 3 segundos de timeout

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1
```

### Multi-Modelo con Routing Basado en Modelo

Enrutar diferentes modelos a proveedores especializados:

```yaml
routing:
  strategy: model_based
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
    llama: ollama
  default_provider: anthropic

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"

  - name: "ollama"
    type: "ollama"
    base_url: "http://localhost:11434"
```

Con esta configuracion:
- Modelos Claude → Anthropic
- Modelos GLM → Z.AI
- Modelos Qwen/Llama → Ollama (local)
- Otros modelos → Anthropic (predeterminado)

## Peso y Prioridad de Proveedor

El peso y la prioridad se especifican en la configuracion de claves del proveedor:

```yaml
providers:
  - name: "example"
    type: "anthropic"
    keys:
      - key: "${API_KEY}"
        weight: 3      # Para weighted-round-robin (mayor = mas trafico)
        priority: 2    # Para failover (mayor = se intenta primero)
        rpm_limit: 60  # Seguimiento de limite de tasa
```

**Nota:** El peso y la prioridad se leen de la **primera clave** en la lista de claves del proveedor.

## Proximos Pasos

- [Referencia de configuracion](/es/docs/configuration/) - Opciones de configuracion completas
- [Vision general de la arquitectura](/es/docs/architecture/) - Como funciona cc-relay internamente

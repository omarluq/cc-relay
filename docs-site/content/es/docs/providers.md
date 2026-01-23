---
title: "Proveedores"
description: "Configure proveedores Anthropic, Z.AI y Ollama en cc-relay"
weight: 5
---

CC-Relay soporta multiples proveedores de LLM a traves de una interfaz unificada. Esta pagina explica como configurar cada proveedor.

## Descripcion General

CC-Relay actua como un proxy entre Claude Code y varios backends de LLM. Todos los proveedores exponen una API de Messages compatible con Anthropic, permitiendo un cambio fluido entre proveedores.

| Proveedor | Tipo | Descripcion | Costo |
|-----------|------|-------------|-------|
| Anthropic | `anthropic` | Acceso directo a la API de Anthropic | Precios estandar de Anthropic |
| Z.AI | `zai` | Modelos GLM de Zhipu AI, compatible con Anthropic | ~1/7 del precio de Anthropic |
| Ollama | `ollama` | Inferencia LLM local | Gratis (computo local) |

**Disponible en Fase 6:** AWS Bedrock, Azure Foundry, Google Vertex AI

## Proveedor Anthropic

El proveedor Anthropic se conecta directamente a la API de Anthropic. Este es el proveedor predeterminado para acceso completo a los modelos Claude.

### Configuracion

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Opcional, usa predeterminado

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60        # Solicitudes por minuto
        tpm_limit: 100000    # Tokens por minuto
        priority: 2          # Mayor = se intenta primero en failover

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```

### Configuracion de API Key

1. Cree una cuenta en [console.anthropic.com](https://console.anthropic.com)
2. Navegue a Settings > API Keys
3. Cree una nueva API key
4. Almacene en variable de entorno: `export ANTHROPIC_API_KEY="sk-ant-..."`

### Soporte de Auth Transparente

El proveedor Anthropic soporta autenticacion transparente para usuarios con suscripcion a Claude Code. Cuando esta habilitado, cc-relay reenvia su token de suscripcion sin cambios:

```yaml
server:
  auth:
    allow_subscription: true
```

```bash
# Su token de suscripcion se reenvia sin cambios
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

Vea [Autenticacion Transparente](/es/docs/configuration/#autenticacion-transparente) para detalles.

## Proveedor Z.AI

Z.AI (Zhipu AI) ofrece modelos GLM a traves de una API compatible con Anthropic. Esto proporciona ahorros significativos (~1/7 del precio de Anthropic) mientras mantiene compatibilidad con la API.

### Configuracion

```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"  # Opcional, usa predeterminado

    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Menor prioridad que Anthropic para failover

    # Mapear nombres de modelos Claude a modelos Z.AI
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"
      "claude-haiku-3-5": "GLM-4.5-Air"

    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"
```

### Configuracion de API Key

1. Cree una cuenta en [open.bigmodel.cn](https://open.bigmodel.cn) (Portal de Desarrolladores Zhipu AI)
2. Navegue a la seccion de API Keys
3. Cree una nueva API key
4. Almacene en variable de entorno: `export ZAI_API_KEY="..."`

### Model Mapping

El model mapping traduce nombres de modelos Anthropic a equivalentes de Z.AI. Cuando Claude Code solicita `claude-sonnet-4-5-20250514`, cc-relay automaticamente redirige a `GLM-4.7`:

```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7 (modelo insignia)
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air (rapido, economico)
  "claude-haiku-3-5-20241022": "GLM-4.5-Air"
  "claude-haiku-3-5": "GLM-4.5-Air"
```

### Comparacion de Costos

| Modelo | Anthropic (por 1M tokens) | Equivalente Z.AI | Costo Z.AI |
|--------|---------------------------|------------------|------------|
| claude-sonnet-4-5 | $3 entrada / $15 salida | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 entrada / $1.25 salida | GLM-4.5-Air | ~$0.04 / $0.18 |

*Los precios son aproximados y pueden cambiar.*

## Proveedor Ollama

Ollama permite inferencia LLM local a traves de una API compatible con Anthropic (disponible desde Ollama v0.14). Ejecute modelos localmente para privacidad, sin costos de API y operacion offline.

### Configuracion

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"  # Opcional, usa predeterminado

    keys:
      - key: "ollama"  # Ollama acepta pero ignora API keys
        priority: 0    # Menor prioridad para failover

    # Mapear nombres de modelos Claude a modelos Ollama locales
    model_mapping:
      "claude-sonnet-4-5-20250514": "qwen3:32b"
      "claude-sonnet-4-5": "qwen3:32b"
      "claude-haiku-3-5-20241022": "qwen3:8b"
      "claude-haiku-3-5": "qwen3:8b"

    models:
      - "qwen3:32b"
      - "qwen3:8b"
      - "codestral:latest"
```

### Configuracion de Ollama

1. Instale Ollama desde [ollama.com](https://ollama.com)
2. Descargue los modelos que desea usar:
   ```bash
   ollama pull qwen3:32b
   ollama pull qwen3:8b
   ollama pull codestral:latest
   ```
3. Inicie Ollama (se ejecuta automaticamente al instalar)

### Modelos Recomendados

Para flujos de trabajo de Claude Code, elija modelos con al menos 32K de contexto:

| Modelo | Contexto | Tamano | Mejor Para |
|--------|----------|--------|------------|
| `qwen3:32b` | 128K | 32B params | Codificacion general, razonamiento complejo |
| `qwen3:8b` | 128K | 8B params | Iteracion rapida, tareas simples |
| `codestral:latest` | 32K | 22B params | Generacion de codigo, codificacion especializada |
| `llama3.2:3b` | 128K | 3B params | Muy rapido, tareas basicas |

### Limitaciones de Funciones

La compatibilidad de Ollama con Anthropic es parcial. Algunas funciones no estan soportadas:

| Funcion | Soportado | Notas |
|---------|-----------|-------|
| Streaming (SSE) | Si | Misma secuencia de eventos que Anthropic |
| Tool calling | Si | Mismo formato que Anthropic |
| Extended thinking | Parcial | `budget_tokens` aceptado pero no aplicado |
| Prompt caching | No | bloques `cache_control` ignorados |
| Entrada PDF | No | No soportado |
| URLs de imagen | No | Solo codificacion Base64 |
| Conteo de tokens | No | `/v1/messages/count_tokens` no disponible |
| `tool_choice` | No | No puede forzar uso de herramienta especifica |

### Docker Networking

Al ejecutar cc-relay en Docker pero Ollama en el host:

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # Usar gateway del host de Docker en lugar de localhost
    base_url: "http://host.docker.internal:11434"
```

Alternativamente, ejecute cc-relay con `--network host`:

```bash
docker run --network host cc-relay
```

## Model Mapping

El campo `model_mapping` traduce nombres de modelos entrantes a modelos especificos del proveedor:

```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # Formato: "modelo-entrante": "modelo-proveedor"
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
```

Cuando Claude Code envia:
```json
{"model": "claude-sonnet-4-5-20250514", ...}
```

CC-Relay redirige a Z.AI con:
```json
{"model": "GLM-4.7", ...}
```

### Consejos de Mapping

1. **Incluir sufijos de version**: Mapear tanto `claude-sonnet-4-5` como `claude-sonnet-4-5-20250514`
2. **Considerar longitud de contexto**: Emparejar modelos con capacidades similares
3. **Probar calidad**: Verificar que la calidad de salida cumple sus necesidades

## Configuracion Multi-Proveedor

Configure multiples proveedores para failover, optimizacion de costos o distribucion de carga:

```yaml
providers:
  # Primario: Anthropic (mayor calidad)
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # Se intenta primero

  # Secundario: Z.AI (economico)
  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Respaldo

  # Terciario: Ollama (local, gratis)
  - name: "ollama"
    type: "ollama"
    enabled: true
    keys:
      - key: "ollama"
        priority: 0  # Ultimo recurso

routing:
  strategy: failover  # Intentar proveedores en orden de prioridad
```

Con esta configuracion:
1. Las solicitudes van primero a Anthropic (prioridad 2)
2. Si Anthropic falla (429, 5xx), intentar Z.AI (prioridad 1)
3. Si Z.AI falla, intentar Ollama (prioridad 0)

Vea [Estrategias de Routing](/es/docs/routing/) para mas opciones.

## Solucion de Problemas

### Conexion Rechazada (Ollama)

**Sintoma:** `connection refused` al conectar con Ollama

**Causas:**
- Ollama no esta corriendo
- Puerto incorrecto
- Problema de Docker networking

**Soluciones:**
```bash
# Verificar si Ollama esta corriendo
ollama list

# Verificar puerto
curl http://localhost:11434/api/version

# Para Docker, usar gateway del host
base_url: "http://host.docker.internal:11434"
```

### Autenticacion Fallida (Z.AI)

**Sintoma:** `401 Unauthorized` de Z.AI

**Causas:**
- API key invalida
- Variable de entorno no configurada
- Key no activada

**Soluciones:**
```bash
# Verificar variable de entorno
echo $ZAI_API_KEY

# Probar key directamente
curl -X POST https://api.z.ai/api/anthropic/v1/messages \
  -H "x-api-key: $ZAI_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"GLM-4.7","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### Modelo No Encontrado

**Sintoma:** errores `model not found`

**Causas:**
- Modelo no configurado en lista `models`
- Entrada faltante en `model_mapping`
- Modelo no instalado (Ollama)

**Soluciones:**
```yaml
# Asegurar que el modelo este listado
models:
  - "GLM-4.7"

# Asegurar que el mapping existe
model_mapping:
  "claude-sonnet-4-5": "GLM-4.7"
```

Para Ollama, verificar que el modelo este instalado:
```bash
ollama list
ollama pull qwen3:32b
```

### Respuesta Lenta (Ollama)

**Sintoma:** Respuestas muy lentas de Ollama

**Causas:**
- Modelo muy grande para el hardware
- GPU no esta siendo usada
- RAM insuficiente

**Soluciones:**
- Usar modelo mas pequeno (`qwen3:8b` en lugar de `qwen3:32b`)
- Verificar GPU habilitada: `ollama run qwen3:8b --verbose`
- Verificar uso de memoria durante inferencia

## Siguientes Pasos

- [Referencia de Configuracion](/es/docs/configuration/) - Opciones completas de configuracion
- [Estrategias de Routing](/es/docs/routing/) - Seleccion de proveedor y failover
- [Monitoreo de Salud](/es/docs/health/) - Circuit breakers y health checks

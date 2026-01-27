---
title: Documentacion
weight: 1
---

Bienvenido a la documentacion de CC-Relay. Esta guia te ayudara a configurar y usar CC-Relay como proxy multi-proveedor para Claude Code y otros clientes LLM.

## Que es CC-Relay?

CC-Relay es un proxy HTTP de alto rendimiento escrito en Go que se ubica entre clientes LLM (como Claude Code) y proveedores LLM. Proporciona:

- **Soporte multi-proveedor**: Anthropic y Z.AI (con mas proveedores planeados)
- **Compatible con API de Anthropic**: Reemplazo directo para acceso directo a la API
- **Streaming SSE**: Soporte completo para respuestas en streaming
- **Multiples metodos de autenticacion**: Soporte para API key y Bearer token
- **Integracion con Claude Code**: Configuracion facil con comando integrado

## Estado Actual

CC-Relay esta en desarrollo activo. Funciones actualmente implementadas:

| Funcion | Estado |
|---------|--------|
| Servidor Proxy HTTP | Implementado |
| Proveedor Anthropic | Implementado |
| Proveedor Z.AI | Implementado |
| Streaming SSE | Implementado |
| Autenticacion API Key | Implementado |
| Autenticacion Bearer Token (Suscripcion) | Implementado |
| Configuracion Claude Code | Implementado |
| Multiples API Keys | Implementado |
| Registro de Depuracion | Implementado |

**Funciones planeadas:**
- Estrategias de enrutamiento (round-robin, failover, basado en costo)
- Limitacion de tasa por API key
- Circuit breaker y seguimiento de salud
- API de gestion gRPC
- Dashboard TUI
- Proveedores adicionales (Ollama, Bedrock, Azure, Vertex)

## Inicio Rapido

```bash
# Instalar
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest

# Inicializar configuracion
cc-relay config init

# Establecer tu API key
export ANTHROPIC_API_KEY="tu-key-aqui"

# Iniciar el proxy
cc-relay serve

# Configurar Claude Code (en otra terminal)
cc-relay config cc init
```

## Navegacion Rapida

- [Comenzar](/es/docs/getting-started/) - Instalacion y primera ejecucion
- [Configuracion](/es/docs/configuration/) - Configuracion de proveedores y opciones
- [Arquitectura](/es/docs/architecture/) - Diseno del sistema y componentes
- [Referencia de API](/es/docs/api/) - Endpoints HTTP y ejemplos

## Secciones de Documentacion

### Comenzar
- [Instalacion](/es/docs/getting-started/#instalacion)
- [Inicio Rapido](/es/docs/getting-started/#inicio-rapido)
- [Comandos CLI](/es/docs/getting-started/#comandos-cli)
- [Pruebas con Claude Code](/es/docs/getting-started/#verificar-que-funciona)
- [Solucion de Problemas](/es/docs/getting-started/#solucion-de-problemas)

### Configuracion
- [Configuracion del Servidor](/es/docs/configuration/#configuracion-del-servidor)
- [Configuracion de Proveedores](/es/docs/configuration/#configuracion-de-proveedores)
- [Autenticacion](/es/docs/configuration/#autenticacion)
- [Configuracion de Logging](/es/docs/configuration/#configuracion-de-logging)
- [Configuraciones de Ejemplo](/es/docs/configuration/#configuraciones-de-ejemplo)

### Arquitectura
- [Vision General del Sistema](/es/docs/architecture/#vision-general-del-sistema)
- [Componentes Principales](/es/docs/architecture/#componentes-principales)
- [Flujo de Solicitudes](/es/docs/architecture/#flujo-de-solicitudes)
- [Streaming SSE](/es/docs/architecture/#streaming-sse)
- [Flujo de Autenticacion](/es/docs/architecture/#flujo-de-autenticacion)

### Referencia de API
- [POST /v1/messages](/es/docs/api/#post-v1messages)
- [GET /v1/models](/es/docs/api/#get-v1models)
- [GET /v1/providers](/es/docs/api/#get-v1providers)
- [GET /health](/es/docs/api/#get-health)
- [Ejemplos de Cliente](/es/docs/api/#ejemplos-curl)

## Necesitas Ayuda?

- [Reportar un problema](https://github.com/omarluq/cc-relay/issues)
- [Discusiones](https://github.com/omarluq/cc-relay/discussions)

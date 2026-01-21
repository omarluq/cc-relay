---
title: Acerca de
type: about
---

## Acerca de CC-Relay

CC-Relay es un proxy HTTP de alto rendimiento escrito en Go que permite a Claude Code y otros clientes LLM conectarse a multiples proveedores a traves de un unico endpoint.

### Objetivos del Proyecto

- **Simplificar el acceso multi-proveedor** - Un proxy, multiples backends
- **Mantener compatibilidad de API** - Reemplazo directo para acceso a la API de Anthropic
- **Habilitar flexibilidad** - Cambia facilmente de proveedor sin cambios en el cliente
- **Soportar Claude Code** - Integracion de primera clase con Claude Code CLI

### Estado Actual

CC-Relay esta en desarrollo activo. Las siguientes funciones estan implementadas:

- Servidor proxy HTTP con compatibilidad de API de Anthropic
- Soporte de proveedores Anthropic y Z.AI
- Soporte completo de streaming SSE
- Autenticacion con API key y Bearer token
- Multiples API keys por proveedor
- Registro de depuracion para inspeccion de solicitudes/respuestas
- Comandos de configuracion de Claude Code

### Funciones Planeadas

- Proveedores adicionales (Ollama, AWS Bedrock, Azure, Vertex AI)
- Estrategias de enrutamiento (round-robin, failover, basado en costo)
- Limitacion de tasa por API key
- Circuit breaker y seguimiento de salud
- API de gestion gRPC
- Dashboard TUI

### Construido Con

- [Go](https://go.dev/) - Lenguaje de programacion
- [Cobra](https://cobra.dev/) - Framework CLI
- [zerolog](https://github.com/rs/zerolog) - Logging estructurado

### Autor

Creado por [Omar Alani](https://github.com/omarluq)

### Licencia

CC-Relay es software de codigo abierto bajo la [Licencia AGPL 3](https://github.com/omarluq/cc-relay/blob/main/LICENSE).

### Contribuir

Las contribuciones son bienvenidas. Consulta el [repositorio de GitHub](https://github.com/omarluq/cc-relay) para:

- [Reportar problemas](https://github.com/omarluq/cc-relay/issues)
- [Enviar pull requests](https://github.com/omarluq/cc-relay/pulls)
- [Discusiones](https://github.com/omarluq/cc-relay/discussions)

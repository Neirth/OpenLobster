# Tests Maestro - OpenLobster Frontend

Especificación y flujos de pruebas end-to-end con [Maestro](https://maestro.dev/) para el frontend de OpenLobster. Cubre **todo** lo que probaría un usuario, incluyendo edge cases.

## Requisitos

- [Maestro CLI](https://maestro.dev/docs/getting-started/installation) instalado
- Frontend corriendo en `http://localhost:5173` (o configurar `MAESTRO_BASE_URL`)
- Backend OpenLobster operativo para flujos que requieren datos

## Estructura

```
tests/manual/
├── README.md                 # Este archivo
├── SPECIFICATION.md          # Especificación completa de todos los tests
├── config.yaml               # Configuración Maestro del proyecto
└── maestro-flows/             # Flujos YAML ejecutables
    ├── 01-navigation/        # Navegación entre vistas
    ├── 02-dashboard/         # Panel principal
    ├── 03-chat/              # Chat y conversaciones
    ├── 04-tasks/             # Tareas programadas
    ├── 05-memory/            # Memoria y grafo
    ├── 06-mcps/              # Servidores MCP y permisos
    ├── 07-skills/            # Skills
    ├── 08-settings/          # Configuración
    ├── 09-auth/              # Autenticación y modales
    └── 10-edge-cases/        # Edge cases y errores
```

## Ejecución

```bash
# Desde la raíz del monorepo
cd apps/frontend

# Iniciar el frontend (en otra terminal)
pnpm dev

# Ejecutar todos los flujos
maestro test tests/manual/maestro-flows/

# Ejecutar un flujo específico
maestro test tests/manual/maestro-flows/01-navigation/01-all-tabs.yaml

# Ejecutar con URL base personalizada
MAESTRO_BASE_URL=http://localhost:5173 maestro test tests/manual/maestro-flows/
```

## Notas

- Los flujos asumen **inglés** como idioma de la UI (textos de `en.json`)
- Para español/chino, ajustar los selectores de texto en los flows
- Algunos flujos requieren datos de prueba (conversaciones, tareas, etc.)
- Los edge cases de BrowserCheck/MobileBlocker requieren emulación de dispositivos

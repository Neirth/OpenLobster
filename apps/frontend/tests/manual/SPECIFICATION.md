# Especificación completa de tests Maestro – OpenLobster Frontend

Este documento define **todo** lo que Maestro debe probar en el frontend de OpenLobster, incluyendo flujos felices, edge cases y estados de error. Sirve como referencia para ejecutar y mantener los tests E2E.

---

## Requisitos previos

- **Backend** en ejecución (para la mayoría de los flujos)
- **Frontend** en `http://localhost:5173` (o `MAESTRO_BASE_URL`)
- **Maestro CLI** instalado: `curl -Ls "https://get.maestro.mobile.dev" | bash`
- **Token de acceso** válido si el backend requiere autenticación (401 → AccessTokenModal)

---

## 1. Navegación

### 1.1 Navegación por tabs (Header)

| Paso | Acción | Verificación |
|------|--------|--------------|
| 1 | Abrir app en `/` | Dashboard visible |
| 2 | Clic en tab "Chat" | URL `/chat`, vista Chat |
| 3 | Clic en tab "Tasks" | URL `/tasks`, vista Tasks |
| 4 | Clic en tab "Memory" | URL `/memory`, vista Memory |
| 5 | Clic en tab "Tools" (MCPs) | URL `/mcps`, vista MCPs |
| 6 | Clic en tab "Skills" | URL `/skills`, vista Skills |
| 7 | Clic en tab "Settings" | URL `/settings`, vista Settings |
| 8 | Clic en tab "Dashboard" | URL `/`, vista Dashboard |

**Selectores:** `Dashboard`, `Chat`, `Tasks`, `Memory`, `Tools`, `Skills`, `Settings` (textos del header).

### 1.2 Acceso directo por URL

| Ruta | Verificación |
|------|--------------|
| `/` | Dashboard, métricas, canales, logs |
| `/chat` | Chat, sidebar conversaciones |
| `/tasks` | Tasks, tabla o empty state |
| `/memory` | Memory, índice o empty state |
| `/mcps` | MCPs, tabs Servers/Built-in/Permissions |
| `/skills` | Skills, grid o empty state |
| `/settings` | Settings, formulario o loading |

### 1.3 Ruta 404

| Paso | Acción | Verificación |
|------|--------|--------------|
| 1 | Navegar a `/ruta-inexistente` | Error404 visible, código 404 |
| 2 | Navegar a `/chat/123` | Error404 (ruta no definida) |

---

## 2. Dashboard

### 2.1 Vista overview (con datos)

- **Health:** OK / ERROR / KO con icono y color
- **Métricas:** Pending Tasks, Completed Tasks, Messages Received, Messages Sent
- **Canales:** lista con nombre, estado (online/degraded/offline)
- **Sistema:** Memory Backend, Uptime, Secrets Backend
- **Conversaciones recientes:** hasta 5, avatar, nombre, canal, badge activo/idle
- **Servidores MCP:** hasta 5, favicon, nombre, transporte, nº herramientas, estado
- **Logs recientes:** panel con scroll, texto plano

### 2.2 Estados vacíos del Dashboard

| Estado | Condición | Texto esperado |
|--------|-----------|----------------|
| Sin canales | `channels` vacío | "No channels configured." |
| Sin sesiones activas | `sessions` vacío | "No active sessions." |
| Sin MCPs | `mcps` vacío | "No MCP servers connected" |
| Logs vacíos | sin logs | "Waiting for logs..." o "No logs available" |

---

## 3. Chat

### 3.1 Estado vacío (sin conversaciones)

- Icono, título "No conversations yet"
- Hint: "Don't be shy — your agent is waiting for you..."
- (Opcional) Botón o CTA si existe

### 3.2 Sin conversación seleccionada

- Mensaje: "Select a conversation to start chatting"
- Textarea y botón Enviar visibles pero área de mensajes vacía

### 3.3 Seleccionar conversación

- Clic en una fila de la lista
- Panel derecho muestra hilo de mensajes
- Header del hilo con nombre de usuario/canal

### 3.4 Enviar mensaje

| Paso | Acción | Verificación |
|------|--------|--------------|
| 1 | Escribir en textarea "Type a message..." | Texto visible |
| 2 | Botón "Send" deshabilitado si vacío | — |
| 3 | Escribir texto, clic "Send" | Mensaje aparece en hilo |
| 4 | Ctrl/Cmd+Enter | Envía mensaje (atajo) |

### 3.5 Adjuntar archivo y emoji

- Botón "Attach file" → input file
- Botón "Insert emoji" → selector con emojis
- Adjuntar archivo → preview con tipo y tamaño
- Insertar emoji → emoji en textarea

### 3.6 Eliminar usuario

| Paso | Acción | Verificación |
|------|--------|--------------|
| 1 | Clic "Delete user" en header del hilo | Modal de confirmación |
| 2 | Modal muestra lista de datos a eliminar | Texto "This action is irreversible" |
| 3 | Input para escribir nombre del usuario | Placeholder visible |
| 4 | Escribir nombre incorrecto, clic "Delete permanently" | Botón deshabilitado |
| 5 | Escribir nombre correcto, clic "Delete permanently" | Usuario eliminado, conversación desaparece |
| 6 | Clic "Cancel" | Modal se cierra, sin cambios |

### 3.7 Edge cases Chat

- **Enviar con textarea vacío:** botón Send deshabilitado
- **Enviar durante `isPending`:** botón deshabilitado
- **Conversación sin mensajes:** "No messages" o icono forum en área de mensajes

---

## 4. Tasks

### 4.1 Estado vacío

- Icono, "No scheduled tasks"
- Hint: "Create a task to automate..."
- Botón "New Task"

### 4.2 Crear tarea one-shot

| Paso | Acción | Verificación |
|------|--------|--------------|
| 1 | Clic "New Task" | Modal Nueva Tarea |
| 2 | Rellenar Task Name, Prompt | Campos aceptan texto |
| 3 | Tipo "One-shot", Schedule ISO o vacío | Vacío = run immediately |
| 4 | Clic "Create Task" | Tarea aparece en tabla |
| 5 | Clic "Cancel" | Modal cierra, sin crear |

### 4.3 Crear tarea cíclica

- Tipo "Cyclic"
- Schedule cron (ej: `0 8 * * *`)
- Crear → tarea en tabla con tipo "Cyclic"

### 4.4 Editar tarea

- Clic botón editar en fila
- Modal Editar con campos pre-rellenados
- Modificar, Guardar → cambios visibles

### 4.5 Eliminar tarea

- Clic botón eliminar
- Modal "Delete Task?" con confirmación
- Clic "Delete" → tarea desaparece
- Clic "Cancel" → sin cambios

### 4.6 Toggle habilitado

- Toggle ON/OFF por tarea
- Estado persiste (optimistic update)

### 4.7 Edge cases Tasks

- **Schedule vacío:** aceptado (run immediately)
- **Schedule inválido:** backend puede fallar; sin validación frontend explícita
- **Campos requeridos vacíos:** `required` HTML bloquea submit
- **Toggle falla:** UI revierte (optimistic)

---

## 5. Memory

### 5.1 Estado vacío

- "No memories yet"
- Hint: "Start a conversation with the agent..."

### 5.2 Sin nodo seleccionado

- "Select an entry to view details"
- Sidebar con búsqueda visible

### 5.3 Búsqueda y selección

- Input "Search memories..."
- Escribir texto → lista filtrada
- Clic en nodo → detalle en área principal
- Grafo visible, conexiones Incoming/Outgoing

### 5.4 Editar nodo

| Paso | Acción | Verificación |
|------|--------|--------------|
| 1 | Clic "Edit Memory Node" | Modal Editar |
| 2 | Cambiar Label, propiedades | Campos editables |
| 3 | Añadir propiedad (key-value) | Nueva fila |
| 4 | Clic "Save" | Cambios guardados |
| 5 | Clic "Cancel" | Sin cambios |

### 5.5 Eliminar nodo

- Clic "Delete"
- Modal "Are you sure you want to delete {name}?"
- Clic "Delete" → nodo desaparece
- Clic "Cancel" → sin cambios

### 5.6 Navegar por conexiones

- Clic en edge en lista de conexiones → cambia nodo seleccionado

### 5.7 Edge cases Memory

- **Nodo sin propiedades:** "No properties. Click Add property to begin."
- **Grafo cargando:** "Loading graph..."

---

## 6. MCPs (Tools)

### 6.1 Tabs

- **Servers:** grid de servidores o empty state
- **Built-in:** capacidades (Browser, Terminal, Subagents, Memory, Filesystem, Sessions, MCP, Audio)
- **Permissions:** usuarios y matriz de herramientas

### 6.2 Estado vacío Servers

- "No MCP servers connected"
- Botones "Marketplace" y "Add MCP Server"

### 6.3 Añadir servidor (modal)

| Paso | Acción | Verificación |
|------|--------|--------------|
| 1 | Clic "Add MCP Server" | Modal con Server Name, Server URL |
| 2 | Nombre vacío o URL vacía | No debe enviar (validación) |
| 3 | Rellenar nombre y URL válida | Clic "Add Server" → servidor conectado |
| 4 | Clic "Cancel" o overlay | Modal cierra |

### 6.4 Marketplace

- Clic "Marketplace" → modal con lista de servidores
- Búsqueda filtra por nombre, empresa, descripción
- Sin resultados → "No results" o icono search_off
- Clic en servidor → detalle, botón "Conectar"
- Conectar → cierra modal, servidor añadido
- Cerrar (X o overlay) → vuelve a MCPs

### 6.5 Gestionar servidor

- Clic "Manage" en tarjeta
- Modal: herramientas, estado, OAuth (si aplica), Desconectar
- OAuth: "Authorize with OAuth" → popup (no automatizable fácilmente)
- Desconectar → confirmación

### 6.6 Built-in detail

- Clic en tarjeta de capacidad (ej: Browser)
- Modal con descripción y lista de herramientas
- Cerrar

### 6.7 Permissions

- **Sin usuarios:** "No users found"
- **Sin usuario seleccionado:** "Select a user to manage their tool permissions"
- **Sin herramientas:** "No MCP tools available"
- Seleccionar usuario → matriz de toggles Allow/Deny
- "Allow All" / "Deny All" → bulk
- Toggle por herramienta

### 6.8 Edge cases MCPs

- **Add Server sin nombre/URL:** no envía, sin mensaje de error explícito
- **Marketplace fetch falla:** mensaje de error
- **OAuth popup cerrado sin completar:** vuelve a idle
- **OAuth callback error:** OAuthCallbackError en popup, postMessage al opener

---

## 7. Skills

### 7.1 Estado vacío

- "No skills found"
- Hint: "Place skill files in workspace/skills folder..."
- Botón "Import Skill"

### 7.2 Importar skill

- Clic "Import Skill" → input file (accept=".skill")
- Seleccionar archivo .skill → skill aparece en grid
- Archivo inválido → `importError` mostrado

### 7.3 Eliminar skill

- Clic eliminar en tarjeta
- Confirmación "Delete?" con Sí / No
- Sí → skill desaparece
- No → sin cambios

---

## 8. Settings

### 8.1 Carga inicial

- Pantalla "Loading configuration" con icono
- Tras cargar → formulario con grupos (General, Capabilities, Database, Memory, etc.)

### 8.2 Guardar configuración

- Modificar campo
- Clic "Save Changes"
- "Saving..." durante guardado
- "Configuration saved successfully!" o mensaje de error

### 8.3 Archivos de workspace

- Tabs: AGENTS.md, SOUL.md, IDENTITY.md
- Cambiar tab → editor con contenido
- Editar texto, "Save" → "Saved!" o error

### 8.4 Enlaces de documentación

- Tabla con Telegram, Discord, WhatsApp, Twilio, Slack
- Enlaces abren en nueva pestaña (verificar que existen)

### 8.5 Edge cases Settings

- **401 al cargar:** AccessTokenModal
- **Load config falla:** "Failed to load configuration from server"
- **Save falla:** mensaje de error
- **Timeout 800ms:** AbortController puede cortar fetch

---

## 9. Autenticación

### 9.1 AccessTokenModal (401)

- **Trigger:** backend devuelve 401
- **Comportamiento:** modal fullscreen, input token
- **Token vacío:** error "Token cannot be empty"
- **Token válido:** guarda en sessionStorage, modal cierra
- **Token inválido:** 401 persiste, modal reaparece
- **No hay botón cerrar:** gate hasta autenticarse

### 9.2 PairingModal

- **Trigger:** evento PairingRequestEvent por suscripción
- **Campos:** Display name, modo Usuario existente / Nuevo usuario
- **Usuario existente:** select con usuarios; debe seleccionar para aprobar
- **Nuevo usuario:** siempre puede aprobar
- **Aprobar:** mutación approvePairing
- **Denegar:** mutación denyPairing
- **Cerrar sin acción:** descarta en frontend, no llama denyPairing

### 9.3 OAuthCallbackError

- **Trigger:** `/?oauth_callback=error&message=...` en popup con window.opener
- **Comportamiento:** muestra mensaje, postMessage al opener, botón "Close window"

---

## 10. Edge cases globales

### 10.1 Formularios

| Caso | Ubicación | Comportamiento esperado |
|------|-----------|-------------------------|
| Token vacío | AccessTokenModal | Error "Token cannot be empty" |
| Token solo espacios | AccessTokenModal | No pasa validación |
| Add MCP sin nombre/URL | McpsView | No envía, sin mensaje (mejorable) |
| Pairing modo existente sin usuario | PairingModal | Botón Aprobar deshabilitado |
| Tasks campos requeridos | TasksView | HTML required bloquea |
| Memory propiedades key/value | MemoryView | required en campos |

### 10.2 Estados vacíos (todos)

- Chat: "No conversations yet"
- Tasks: "No scheduled tasks"
- Memory: "No memories yet"
- MCPs Servers: "No MCP servers connected"
- MCPs Permissions: "No users found" / "Select a user"
- Skills: "No skills found"
- Dashboard: "No channels", "No active sessions", "No logs"
- Marketplace búsqueda: sin resultados → "No results"

### 10.3 Modales

- Clic en overlay → cierra modal (excepto AccessTokenModal que es gate)
- Botón X/Close → cierra
- Escape: no implementado en Modal genérico

### 10.4 Botones deshabilitados

- Send (Chat): vacío o isPending
- Create Task: durante isPending
- Add Server: durante isPending
- Delete user: nombre no coincide o isPending
- Pairing Aprobar: modo existente sin usuario seleccionado

### 10.5 BrowserCheck y MobileBlocker

- **BrowserCheck:** sin Proxy, async, fetch, grid, CSS vars, ES6 → incompatible
- **MobileBlocker:** viewport < 1024px, user-agent móvil, touch-only → bloqueado
- Solo desktop soportado

### 10.6 WebSocket

- Desconexión → indicador rojo, reconexión automática
- Mensaje mal formado → no crash
- Error → "Connection error"

---

## 11. Resumen de flujos Maestro

| Carpeta | Flujos | Descripción |
|---------|--------|-------------|
| 01-navigation | 3 | Tabs, URL directa, 404 |
| 02-dashboard | 2 | Overview, empty states |
| 03-chat | 5 | Empty, select, send, emoji/attach, delete user |
| 04-tasks | 4 | Empty, create one-shot, create cyclic, toggle/edit/delete |
| 05-memory | 3 | Empty, search/select, edit/delete |
| 06-mcps | 6 | Empty, tabs, add server, marketplace, built-in, permissions |
| 07-skills | 3 | Empty, import/delete, delete confirm |
| 08-settings | 3 | Loading, tabs/files, doc links |
| 09-auth | 4 | Access token, validation, pairing, approve/deny |
| 10-edge-cases | 8 | Validaciones, empty states, modales, send disabled, etc. |

**Total:** ~40 flujos YAML.

---

## 12. Ejecución

```bash
# Desde raíz del monorepo
cd apps/frontend
pnpm dev   # en otra terminal

# Ejecutar todos los flujos
maestro test tests/manual/maestro-flows

# Ejecutar solo navegación
maestro test tests/manual/maestro-flows/01-navigation

# Con URL personalizada
MAESTRO_BASE_URL=https://staging.example.com maestro test tests/manual/maestro-flows
```

---

## Referencias

- [Maestro Docs](https://maestro.mobile.dev/)
- [Maestro Web Testing](https://maestro.dev/)
- Playwright config: `apps/frontend/playwright.config.ts` (baseURL: localhost:5173)

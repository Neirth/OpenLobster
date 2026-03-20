# OpenLobster — Asistente de IA Personal

<p align="center">
    <picture>
        <source media="(prefers-color-scheme: light)" srcset="https://placehold.co/1600x200/ffffff/000000?text=OpenLobster&font=raleway">
         <img src="https://placehold.co/800x200/0b6e4f/ffffff?text=OpenLobster&font=raleway" alt="OpenLobster" width="800">
    </picture>
</p>

<p align="center">
  <strong>Asistente de IA personal y auto-alojado — se ejecuta donde quieres, se conecta a los canales que usas.</strong>
</p>

<p align="center">
  <a href="README.en.md">English</a> •
  <a href="README.es.md">Español</a> •
  <a href="README.zh.md">简体中文</a>
</p>

<p align="center">
  <a href="https://github.com/Neirth/OpenLobster/actions/workflows/release.docker-images.yaml?branch=main"><img src="https://img.shields.io/github/actions/workflow/status/Neirth/OpenLobster/release.docker-images.yaml?branch=master&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/Neirth/OpenLobster/releases"><img src="https://img.shields.io/github/v/release/Neirth/OpenLobster?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="https://neirth.gitbook.io/openlobster"><img src="https://img.shields.io/badge/Docs-GitBook-blue?style=for-the-badge" alt="Docs"></a>
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/License-GPLv3-blue.svg?style=for-the-badge" alt="GPLv3 License"></a>
  <a href="https://discord.gg/Qx9eJcZH5v"><img src="https://img.shields.io/badge/Discord-%235865F2.svg?style=for-the-badge&logo=discord&logoColor=white" alt="Discord"></a>
  <a href="https://github.com/Neirth/OpenLobster/discussions"><img src="https://img.shields.io/badge/Q%26A-1E232A?style=for-the-badge&logo=github&logoColor=white" alt="GitHub Discussions"></a>
</p>

> [!NOTE]
> **¿Migrando desde OpenClaw?** Una guía de migración paso a paso está disponible en [Discussions #44](https://github.com/Neirth/OpenLobster/discussions/44).


Un fork de OpenClaw elaborado con opinión que realmente aborda las cosas de las que la gente se ha estado quejando desde que el proyecto explotó.

OpenClaw tuvo su momento — agente de IA auto-alojado, mucho hype, crecimiento rápido. Luego la comunidad de seguridad le echó un vistazo y se puso feo muy rápido: un lote de CVE que llenó toda una página en RedPacket, y un mercado de habilidades (ClawHub) donde el 26% de las habilidades tenía al menos una vulnerabilidad. El sistema de memoria era un archivo MEMORY.md que explotaba con sesiones concurrentes. El "programador" (scheduler) era un demonio de latidos que se despertaba cada 30 minutos para leer una lista HEARTBEAT.md. El soporte multi-usuario era básicamente inexistente — la documentación literalmente decía "solo la sesión principal escribe en MEMORY.md, previniendo conflictos de sesiones paralelas" como si eso fuera una característica.

Este fork comenzó como un arreglo personal para todo eso y creció desde ahí.

---

## Qué cambió (y por qué)

* **Memoria** — MEMORY.md y una carpeta de archivos markdown no es un sistema de memoria, es una wiki. OpenLobster usa una base de datos de grafos adecuada (Neo4j) donde el agente construye nodos, bordes y relaciones tipificadas a medida que habla con las personas. Puedes navegar por ella y editarla desde la interfaz de usuario. También existe un backend de archivos para uso local que no requiere ejecutar Neo4j.

* **Multi-usuario** — En OpenClaw, la memoria curada solo se cargaba en la "sesión privada, principal" y nunca en contextos de grupo. No existía el concepto de usuarios separados con historiales separados. Aquí, cada usuario a través de cada canal es una entidad de primera clase con su propio historial de conversación, sus propios permisos de herramientas y su propio flujo de emparejamiento (pairing). Un usuario de Telegram y un usuario de Discord pueden hablar con el mismo agente sin interferirse mutuamente.

* **Programador (Scheduler)** — El bucle de latidos leyendo un archivo HEARTBEAT.md cada 30 minutos ha desaparecido. Hay un verdadero programador de tareas con expresiones cron para trabajos recurrentes y fechas/horas ISO 8601 para tareas únicas. El estado, las horas de próxima ejecución y los registros son visibles en el panel de control.

* **MCP** — La integración MCP de OpenClaw era esencialmente una demo. OpenLobster se conecta a cualquier servidor HTTP MCP Streamable, maneja el flujo completo de OAuth 2.1, te permite navegar por las herramientas por servidor y te da una matriz de permisos por usuario para que controles exactamente qué puede hacer cada uno. También hay un mercado (marketplace) para integraciones de un clic.

* **Seguridad** — Esta es la gran diferencia. OpenClaw se distribuyó con la autenticación desactivada por defecto, que es cómo terminas con 40,000 instancias expuestas en Censys. Aquí, la autenticación del panel de control está activada por defecto detrás de un token portador (`OPENLOBSTER_GRAPHQL_AUTH_TOKEN`). La configuración y los secretos se cifran en el disco. Las claves API y los tokens de los canales se almacenan en un backend de secretos (archivo cifrado u OpenBao), no en un YAML en texto plano. Las variables de entorno `OPENLOBSTER_*` nunca se filtran a herramientas de consola. ¿El CVE que permitía a llamadas no autenticadas golpear la API del agente directamente? No es una preocupación aquí.

* **Backend** — OpenClaw era Node.js/TypeScript. Todo el backend ha sido reescrito en Go. Eso significa un único binario estático sin dependencia de tiempo de ejecución, inicio más rápido, menor uso de memoria y una API GraphQL adecuada a través de gqlgen. También hace que el despliegue sea significativamente más simple — sin npm, sin fijar de versión de Node, sin depender de \`node_modules\`.

* **UI** — La interfaz web fue construida teniendo en cuenta su utilidad real. El primer inicio te lleva a un asistente de configuración de cero hasta tener un agente en ejecución sin tocar un archivo de configuración. La configuración es un formulario dinámico que se ajusta basándose en lo que habilitas — solo ves los campos que importan para tu situación. Todo lo que de otra manera necesitarías editar en YAML es accesible desde el navegador.

> [!NOTE]
> **Se necesitan contribuyentes** Estoy pensando en añadir mantenedores a este repositorio. Lo estoy discutiendo en [Discussions #68](https://github.com/Neirth/OpenLobster/discussions/68).

## Stack

| Capa | Tecnología |
| ----- | ---- |
| Frontend | SolidJS + Vite, CSS vanilla |
| Backend | Go, GraphQL (gqlgen) |
| Base de Datos | SQLite / PostgreSQL / MySQL |
| Memoria | Archivo (GML) o Neo4j |
| Secretos | Archivo encriptado o OpenBao |
| Canales | Telegram, Discord, WhatsApp, Slack, Twilio SMS |
| IA | OpenAI, Anthropic, Ollama, OpenRouter, Docker Model Runner, compatibles con OpenAI |

## Inicio básico

```bash
# Instalar dependencias
pnpm install

# Construir frontend + backend (frontend embebido en el binario)
pnpm build --filter=@openlobster/backend

# Construir solo el frontend
pnpm build --filter=@openlobster/frontend

# Construir ambos
pnpm build

# Ejecutar
./dist/openlobster
```

El panel de control web estará en `http://127.0.0.1:8080`. En el primer inicio, el asistente de instalación te guiará con la configuración inicial.

## Docker

```bash
docker run -p 8080:8080 \
  -e OPENLOBSTER_GRAPHQL_HOST=0.0.0.0 \
  -e OPENLOBSTER_GRAPHQL_AUTH_TOKEN=tu-token-secreto \
  -v ~/.openlobster/data:/app/data \
  -v ~/.openlobster/workspace:/app/workspace \
  -d ghcr.io/neirth/openlobster/openlobster:latest
```

Revisa `.docker/` para ver los Dockerfiles disponibles (`Dockerfile.basic` para una build mínima, `Dockerfile.static` para un binario estático).

## Configuración

La configuración vive en el panel principal bajo Herramientas, pero puedes definir todo por medio de variables de entorno con el prefijo `OPENLOBSTER_`. Viper los mapeara automáticamente (los puntos en las llaves YAML pasarán a guiones bajos).

```bash
# Ejemplo minimalista
OPENLOBSTER_AGENT_NAME=mi-agente
OPENLOBSTER_DATABASE_DRIVER=sqlite
OPENLOBSTER_DATABASE_DSN=./data/openlobster.db
OPENLOBSTER_GRAPHQL_AUTH_TOKEN=tu-token-secreto

# Proveedor IA (elige uno)
OPENLOBSTER_PROVIDERS_OLLAMA_ENDPOINT=http://localhost:11434
OPENLOBSTER_PROVIDERS_OLLAMA_DEFAULT_MODEL=llama3.2:latest
```

<details>
<summary>Referencia completa de variables de entorno</summary>

| Variable | Llave YAML | Descripción |
| -------- | -------- | ----------- |
| `OPENLOBSTER_AGENT_NAME` | `agent.name` | Nombre visible del agente |
| `OPENLOBSTER_DATABASE_DRIVER` | `database.driver` | `sqlite`, `postgres`, `mysql` |
| `OPENLOBSTER_DATABASE_DSN` | `database.dsn` | Cadena de conexión |
| `OPENLOBSTER_DATABASE_MAX_OPEN_CONNS` | `database.max_open_conns` | Conexiones máximas abiertas |
| `OPENLOBSTER_DATABASE_MAX_IDLE_CONNS` | `database.max_idle_conns` | Conexiones máximas inactivas |
| `OPENLOBSTER_MEMORY_BACKEND` | `memory.backend` | `file` o `neo4j` |
| `OPENLOBSTER_MEMORY_FILE_PATH` | `memory.file.path` | Ruta para el backend de archivo |
| `OPENLOBSTER_MEMORY_NEO4J_URI` | `memory.neo4j.uri` | ej. `bolt://localhost:7687` |
| `OPENLOBSTER_MEMORY_NEO4J_USER` | `memory.neo4j.user` | Usuario de Neo4j |
| `OPENLOBSTER_MEMORY_NEO4J_PASSWORD` | `memory.neo4j.password` | Contraseña de Neo4j |
| `OPENLOBSTER_SECRETS_BACKEND` | `secrets.backend` | `file` u `openbao` |
| `OPENLOBSTER_SECRETS_FILE_PATH` | `secrets.file.path` | Ruta para secretos de archivo |
| `OPENLOBSTER_SECRETS_OPENBAO_URL` | `secrets.openbao.url` | URL del servidor OpenBao |
| `OPENLOBSTER_SECRETS_OPENBAO_TOKEN` | `secrets.openbao.token` | Token de autenticación de OpenBao |
| `OPENLOBSTER_PROVIDERS_OPENAI_API_KEY` | `providers.openai.api_key` | Clave de OpenAI |
| `OPENLOBSTER_PROVIDERS_OPENAI_MODEL` | `providers.openai.model` | ej. `gpt-4o` |
| `OPENLOBSTER_PROVIDERS_OPENAI_BASE_URL` | `providers.openai.base_url` | URL base personalizada |
| `OPENLOBSTER_PROVIDERS_OPENROUTER_API_KEY` | `providers.openrouter.api_key` | Clave de OpenRouter |
| `OPENLOBSTER_PROVIDERS_OPENROUTER_DEFAULT_MODEL` | `providers.openrouter.default_model` | ej. `openai/gpt-4o` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_ENDPOINT` | `providers.ollama.endpoint` | ej. `http://localhost:11434` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_DEFAULT_MODEL` | `providers.ollama.default_model` | ej. `llama3.2:latest` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_API_KEY` | `providers.ollama.api_key` | Autenticación opcional |
| `OPENLOBSTER_PROVIDERS_ANTHROPIC_API_KEY` | `providers.anthropic.api_key` | Clave de Anthropic |
| `OPENLOBSTER_PROVIDERS_ANTHROPIC_MODEL` | `providers.anthropic.model` | ej. `claude-sonnet-4-6` |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_API_KEY` | `providers.openaicompat.api_key` | Clave compatible con OpenAI |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_BASE_URL` | `providers.openaicompat.base_url` | URL base |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_MODEL` | `providers.openaicompat.model` | Nombre del modelo |
| `OPENLOBSTER_PROVIDERS_DOCKER_MODEL_RUNNER_ENDPOINT` | `providers.docker_model_runner.endpoint` | Endpoint de DMR |
| `OPENLOBSTER_PROVIDERS_DOCKER_MODEL_RUNNER_DEFAULT_MODEL` | `providers.docker_model_runner.default_model` | Modelo de DMR |
| `OPENLOBSTER_GRAPHQL_ENABLED` | `graphql.enabled` | Habilitar la API GraphQL |
| `OPENLOBSTER_GRAPHQL_PORT` | `graphql.port` | Por defecto `8080` |
| `OPENLOBSTER_GRAPHQL_HOST` | `graphql.host` | Por defecto `127.0.0.1` |
| `OPENLOBSTER_GRAPHQL_BASE_URL` | `graphql.base_url` | URL pública para callbacks de OAuth |
| `OPENLOBSTER_GRAPHQL_AUTH_ENABLED` | `graphql.auth_enabled` | Requerir token para el panel de control |
| `OPENLOBSTER_GRAPHQL_AUTH_TOKEN` | `graphql.auth_token` | Token portador (Bearer token) |
| `OPENLOBSTER_LOGGING_LEVEL` | `logging.level` | `debug`, `info`, `warn`, `error` |
| `OPENLOBSTER_LOGGING_PATH` | `logging.path` | Directorio de registros |
| `OPENLOBSTER_WORKSPACE_PATH` | `workspace.path` | Directorio del espacio de trabajo |
| `OPENLOBSTER_CONFIG_ENCRYPT` | — | `1` (por defecto) encripta la config, `0` usa YAML normal |

</details>

## Canales

El agente habla con los usuarios dondequiera que ya estén. Habilita cualquier combinación en Ajustes (Settings) o a través de variables de entorno.

- **Telegram** — Crea un bot con `@BotFather`, pega el token. Funciona en mensajes directos (DMs) y grupos.
- **Discord** — Crea un bot en el Developer Portal, invítalo a tu servidor. Funciona en DMs y canales.
- **Slack** — Aplicación en Socket Mode con un token de bot (`xoxb-`) y un token de app (`xapp-`).
- **WhatsApp** — API de Negocios de WhatsApp (WhatsApp Business API) a través del Meta Business Suite.
- **Twilio SMS** — SMS estándar a través de Twilio.

## Documentación de usuario

La carpeta `docs/` tiene una guía completa de usuario para la interfaz web — panel de control, chat, navegador de memoria, gestión de MCP, habilidades, tareas y ajustes. Estructurada para GitBook pero funciona también como markdown puro.

## Preguntas Frecuentes (FAQ)

**¿Funciona esto con las configuraciones de OpenClaw?**

No. La arquitectura es lo suficientemente diferente como para que las configuraciones de OpenClaw no se correspondan limpiamente. Además, el modelo de permisos, la integración MCP y la forma en que el agente accede a las herramientas han cambiado, haciendolas incompatibles. Tendrás que migrar manualmente.

**¿Puedo ejecutarlo sin Neo4j?**

Sí. Establece `OPENLOBSTER_MEMORY_BACKEND=file` y apunta `OPENLOBSTER_MEMORY_FILE_PATH` a un directorio. El backend de archivo almacena el grafo en formato GML de forma local. Es perfectamente utilizable para despliegues personales; Neo4j está ahí cuando necesitas múltiples instancias o enrutamiento de consultas con gráficas más complejas.

### ¿Puedo ejecutar esto en dispositivos pequeños?

Sí. Un sólo binario de Go, un inicio rapidísimo.

**Especificaciones reales (medidas):**
- Tiempo de inicio: 200ms (vs ~2-3s para Node.js de OpenClaw)
- RAM: 30MB con todos los servicios cargados (vs ~150MB+ para OpenClaw)
- Tamaño del binario: ~66MB (vs 200MB+ for Node.js + node_modules)

Perfecto para:
- Raspberry Pi 3/4
- VPS con 512MB RAM
- NAS con recursos ajustados
- Incluso en el LicheeRV Nano de $15 (RISC-V)

**¿Puedo usar cualquier proveedor de IA?**

OpenAI, Anthropic, Ollama, OpenRouter, Docker Model Runner, y cualquier punto de enlace (endpoint) compatible con OpenAI están todos soportados. Configura el que tú quieras en Ajustes o a través de variables de entorno. Solo puedes tener un proveedor activo a la vez.

**¿Es pública la API de GraphQL?**

Por defecto, sí — la API está abierta. Para protegerla, establece `OPENLOBSTER_GRAPHQL_AUTH_TOKEN` a un secreto fuerte. Una vez establecido, cada requerimiento a la API y el panel requiere su inclusión como un token portador (bearer token). Si lo vas a exponer a internet, esta debe ser tu primera medida principal. Ten en cuenta que es para consumo por la interfaz de usuario, y su diseño podría cambiar sin ser versionado.

**Soy una empresa, ¿Cómo puedo añadir mi servidor MCP al marketplace?**

Abre un pull request para añadir tu servidor a `apps/frontend/public/marketplace.json`. Si tu empresa es patrocinadora del proyecto, el PR será revisado y fusionado. Si no, se mantendrá abierto — agradecemos las aportaciones, pero sin alguna forma de apoyo no podemos comprometernos a analizar y mantener ingresos de terceros gratuitos. Los servidores de MCP que fueron añadidos en los primeros dias fueron de buena fé y no se aplican estas reglas.

**Quiero una integración de servidor MCP concreta, pero no estoy asociado a ninguna empresa, ¿Puedo pedirla?**

Abre un ticket de ayuda informando donde la deseas y qué propósitos persigue. Si acumula la cantidad adecuada de apoyo comunitario se añadirá gratis para ti y los demas — no hay requisitos económicos de por medio.

**¿Qué significa "emparejamiento" (pairing)?**

Cuando un usuario interactúa con tu agente por primera vez mediante cualquier origen, es conducido a procesar un ciclo que vincule la plataforma nativa del sujeto (El Telegram o Discord del usuario, por poner ejemplo) a una cuenta particular en el servidor que corre OpenLobster, abriendo el camino hacia las credenciales individualizadas operables de ese usuario particular del medio.

**¿Cómo actualizo?**

Traete la nueva versión de los ficheros en formato binarios o de imagen docker y después dale inicio (restart) a las secuencias. Si es de menester realizar migraciones operables de base de datos se llevarán de frente sin aviso de retrazos adicionales.

## Licencia

Consulta [LICENSE.md](LICENSE.md) para más detalles.
